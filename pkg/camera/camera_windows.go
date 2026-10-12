package camera

// Media Foundation through COM vtables with syscall, no cgo. A source reader delivers
// the frames synchronously: it picks the camera's mode (preferably MJPEG, needed for
// several cameras on one USB 2.0 port), decodes it and converts to RGB32 (BGRX), all 30
// frames per second: the development board's cameras offer no lower rate.
// A goroutine per camera waits for frames in ReadSample; nothing interrupts that, so
// Close sets a flag and the goroutine ends with the next frame.

import (
	"errors"
	"fmt"
	"image"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type platform struct {
	source, reader *comObject
	width, height  int
	stride         int           // bytes per row, negative for frames stored bottom-up
	interval       time.Duration // minimum time between delivered frames, 0 for all
	stopped        chan struct{} // closed when the read loop has ended
	closing        atomic.Bool   // ends the read loop
}

// comObject is a COM interface pointer: its first word points to the method table.
type comObject struct {
	vtbl *[64]uintptr
}

// hresult is a failed COM call.
type hresult uint32

func (h hresult) Error() string {
	switch h {
	case eAccessDenied:
		return "access denied: allow camera access for desktop apps in Settings → Privacy & security → Camera"
	case mfEDeviceInvalidated:
		return "camera is gone (unplugged?)"
	}
	return fmt.Sprintf("HRESULT 0x%08X", uint32(h))
}

// call calls the method with the given index of the method table.
//
//go:uintptrescapes
func (o *comObject) call(method int, args ...uintptr) error {
	hr, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	if int32(hr) < 0 {
		return hresult(hr)
	}
	return nil
}

func (o *comObject) release() {
	if o != nil {
		_ = o.call(methodRelease)
	}
}

// queryInterface returns the object's interface iid, nil if it has none.
func (o *comObject) queryInterface(iid *windows.GUID) *comObject {
	var p *comObject
	if o.call(methodQueryInterface, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&p))) != nil {
		return nil
	}
	return p
}

// Method table indexes. Every interface starts with the three of IUnknown;
// IMFActivate, IMFMediaType and IMFSample start with the 33 of IMFAttributes.
const (
	methodQueryInterface = 0
	methodRelease        = 2

	attrGetUINT32          = 7  // IMFAttributes
	attrGetUINT64          = 8  //
	attrGetGUID            = 10 //
	attrGetAllocatedString = 13 //
	attrSetUINT32          = 21 //
	attrSetGUID            = 24 //
	activateObject         = 33 // IMFActivate

	sourceShutdown = 12 // IMFMediaSource

	readerGetNativeMediaType  = 5 // IMFSourceReader
	readerGetCurrentMediaType = 6
	readerSetCurrentMediaType = 7
	readerReadSample          = 9

	sampleConvertToContiguousBuffer = 41 // IMFSample

	bufferLock   = 3 // IMFMediaBuffer
	bufferUnlock = 4

	controlSet = 4 // IAMCameraControl, IAMVideoProcAmp
	controlGet = 5
)

const (
	mfVersion           = 0x00020070 // MF_VERSION
	mfStartupLite       = 1          // MFSTARTUP_LITE, no sockets
	coinitMultithreaded = 0          // COINIT_MULTITHREADED
	firstVideoStream    = 0xFFFFFFFC // MF_SOURCE_READER_FIRST_VIDEO_STREAM

	readerError       = 0x1 // MF_SOURCE_READERF_ERROR
	readerEndOfStream = 0x2 // MF_SOURCE_READERF_ENDOFSTREAM

	cameraControlExposure = 4 // CameraControl_Exposure
	videoProcWhiteBalance = 7 // VideoProcAmp_WhiteBalance
	controlFlagsAuto      = 1 // CameraControl_Flags_Auto, VideoProcAmp_Flags_Auto

	eAccessDenied        hresult = 0x80070005
	mfENoMoreTypes       hresult = 0xC00D36B9
	mfEDeviceInvalidated hresult = 0xC00D3EA2 // MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED
)

func guid(s string) *windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return &g
}

var (
	devSourceType         = guid("c60ac5fe-252a-478f-a0ef-bc8fa5f7cad3") // MF_DEVSOURCE_ATTRIBUTE_SOURCE_TYPE
	devSourceTypeVideo    = guid("8ac3587a-4ae7-42d8-99e0-0a6013eef90f") // …_VIDCAP_GUID
	devFriendlyName       = guid("60d0e559-52f8-4fa2-bbce-acdb34a8ec01") // MF_DEVSOURCE_ATTRIBUTE_FRIENDLY_NAME
	devSymbolicLink       = guid("58f0aad8-22bf-4f8a-bb3d-d2c4978c6e2f") // …_VIDCAP_SYMBOLIC_LINK
	readerVideoProcessing = guid("0f81da2c-b537-4672-a8b2-a681b17307a3") // MF_SOURCE_READER_ENABLE_ADVANCED_VIDEO_PROCESSING
	mtMajorType           = guid("48eba18e-f8c9-4687-bf11-0a74c9f96a8f") // MF_MT_MAJOR_TYPE
	mtSubtype             = guid("f7e34c9a-42e8-4714-b74b-cb29d72c35e5") // MF_MT_SUBTYPE
	mtFrameSize           = guid("1652c33d-d6b2-4012-b834-72030849a37d") // MF_MT_FRAME_SIZE
	mtFrameRate           = guid("c459a2e8-3d2c-4e44-b132-fee5156c7bb0") // MF_MT_FRAME_RATE
	mtDefaultStride       = guid("644b4e48-1e02-4516-b0eb-c01ca9d49ac6") // MF_MT_DEFAULT_STRIDE
	mediaTypeVideo        = guid("73646976-0000-0010-8000-00aa00389b71") // MFMediaType_Video
	videoFormatRGB32      = guid("00000016-0000-0010-8000-00aa00389b71") // MFVideoFormat_RGB32
	videoFormatMJPG       = guid("47504a4d-0000-0010-8000-00aa00389b71") // MFVideoFormat_MJPG
	iidMediaSource        = guid("279a808d-aec7-40c8-9c6b-a6b492c78a66") // IMFMediaSource
	iidCameraControl      = guid("c6e13370-30ac-11d0-a18c-00a0c9118956") // IAMCameraControl
	iidVideoProcAmp       = guid("c6e13360-30ac-11d0-a18c-00a0c9118956") // IAMVideoProcAmp
)

var (
	mfplat                             = windows.NewLazySystemDLL("mfplat.dll")
	mf                                 = windows.NewLazySystemDLL("mf.dll")
	mfreadwrite                        = windows.NewLazySystemDLL("mfreadwrite.dll")
	procMFStartup                      = mfplat.NewProc("MFStartup")
	procMFCreateAttributes             = mfplat.NewProc("MFCreateAttributes")
	procMFCreateMediaType              = mfplat.NewProc("MFCreateMediaType")
	procMFEnumDeviceSources            = mf.NewProc("MFEnumDeviceSources")
	procMFCreateSourceReaderFromSource = mfreadwrite.NewProc("MFCreateSourceReaderFromMediaSource")

	startupOnce sync.Once
	startupErr  error
)

// callProc calls a Media Foundation function that returns an HRESULT.
//
//go:uintptrescapes
func callProc(p *windows.LazyProc, args ...uintptr) error {
	if err := p.Find(); err != nil {
		return err
	}
	hr, _, _ := syscall.SyscallN(p.Addr(), args...)
	if int32(hr) < 0 {
		return hresult(hr)
	}
	return nil
}

// withCOM runs fn on a fixed thread of the multithreaded apartment, as COM expects.
// Media Foundation is started once and stays running.
func withCOM[T any](fn func() (T, error)) (T, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var zero T
	// S_FALSE: COM was already initialized on this thread; it is still counted.
	if err := windows.CoInitializeEx(0, coinitMultithreaded); err != nil && err != syscall.Errno(1) {
		return zero, fmt.Errorf("initializing COM: %w", err)
	}
	defer windows.CoUninitialize()
	startupOnce.Do(func() { startupErr = callProc(procMFStartup, mfVersion, mfStartupLite) })
	if startupErr != nil {
		return zero, fmt.Errorf("starting Media Foundation: %w", startupErr)
	}
	return fn()
}

func newAttributes() (*comObject, error) {
	var a *comObject
	return a, callProc(procMFCreateAttributes, uintptr(unsafe.Pointer(&a)), 1)
}

func getString(a *comObject, key *windows.GUID) string {
	var s *uint16
	var n uint32
	if a.call(attrGetAllocatedString, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&s)), uintptr(unsafe.Pointer(&n))) != nil {
		return ""
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(s))
	return windows.UTF16PtrToString(s)
}

func getGUID(a *comObject, key *windows.GUID) windows.GUID {
	var g windows.GUID
	_ = a.call(attrGetGUID, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&g)))
	return g
}

// getPair reads a UINT64 attribute that packs two numbers, e.g. width and height.
func getPair(a *comObject, key *windows.GUID) (hi, lo int) {
	var v uint64
	_ = a.call(attrGetUINT64, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&v)))
	return int(v >> 32), int(uint32(v))
}

// modelPattern finds the USB ids in a symbolic link, e.g. `\\?\usb#vid_1e45&pid_8022&mi_00#…`.
var modelPattern = regexp.MustCompile(`(?i)vid_([0-9a-f]{4})&pid_([0-9a-f]{4})`)

// eachDevice calls fn for every camera until it returns true. The activation object
// creates the camera's media source; it is released afterwards.
func eachDevice(fn func(d Device, activate *comObject) bool) error {
	attrs, err := newAttributes()
	if err != nil {
		return err
	}
	defer attrs.release()
	if err := attrs.call(attrSetGUID, uintptr(unsafe.Pointer(devSourceType)), uintptr(unsafe.Pointer(devSourceTypeVideo))); err != nil {
		return err
	}
	var list **comObject
	var count uint32
	if err := callProc(procMFEnumDeviceSources, uintptr(unsafe.Pointer(attrs)), uintptr(unsafe.Pointer(&list)), uintptr(unsafe.Pointer(&count))); err != nil {
		return err
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(list))
	activates := unsafe.Slice(list, count)
	defer func() {
		for _, a := range activates {
			a.release()
		}
	}()
	for _, a := range activates {
		d := Device{ID: getString(a, devSymbolicLink), Name: getString(a, devFriendlyName)}
		if m := modelPattern.FindStringSubmatch(d.ID); m != nil {
			d.Model = fmt.Sprintf("%s:%s", m[1], m[2])
		}
		if fn(d, a) {
			break
		}
	}
	return nil
}

// Devices lists the connected cameras.
func Devices() ([]Device, error) {
	return withCOM(func() ([]Device, error) {
		devices := []Device{}
		err := eachDevice(func(d Device, _ *comObject) bool {
			devices = append(devices, d)
			return false
		})
		return devices, err
	})
}

// Open starts delivering frames of the device with the given ID, its symbolic link,
// e.g. `\\?\usb#vid_1e45&pid_8022&mi_00#7&3549ee4f&0&0000#{…}\global`.
func Open(id string, opts Options) (*Camera, error) {
	c, err := withCOM(func() (*Camera, error) {
		var c *Camera
		var err error
		if enumErr := eachDevice(func(d Device, activate *comObject) bool {
			if d.ID != id {
				return false
			}
			c = newCamera(d)
			err = activate.call(activateObject, uintptr(unsafe.Pointer(iidMediaSource)), uintptr(unsafe.Pointer(&c.source)))
			return true
		}); enumErr != nil {
			return nil, enumErr
		}
		if c == nil {
			return nil, fmt.Errorf("no camera with id %q", id)
		}
		if err == nil {
			err = c.start(opts)
		}
		if err != nil {
			c.release()
			return nil, fmt.Errorf("camera %s: %w", c.Device.Name, err)
		}
		return c, nil
	})
	if err != nil {
		return nil, err
	}
	if opts.FPS > 0 {
		c.interval = time.Second / time.Duration(opts.FPS)
	}
	c.stopped = make(chan struct{})
	go c.loop()
	return c, nil
}

// mode is one of the camera's native media types.
type mode struct {
	index         uint32
	width, height int
	stride        int // bytes per row, negative for frames stored bottom-up
	fps           float64
	mjpeg         bool
}

// better reports whether m suits the wanted size better than o: the closest size,
// then MJPEG, then the most frames per second.
func (m mode) better(o mode, width, height int) bool {
	dist := func(m mode) int { return abs(m.width-width) + abs(m.height-height) }
	if dist(m) != dist(o) {
		return dist(m) < dist(o)
	}
	if m.mjpeg != o.mjpeg {
		return m.mjpeg
	}
	return m.fps > o.fps
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (c *Camera) start(opts Options) error {
	attrs, err := newAttributes()
	if err != nil {
		return err
	}
	defer attrs.release()
	// Lets the reader convert the decoded frames to RGB32. The advanced converter needs
	// half the CPU of the basic one (MF_SOURCE_READER_ENABLE_VIDEO_PROCESSING), measured
	// with 3 cameras: 8 instead of 17 % of a core.
	if err := attrs.call(attrSetUINT32, uintptr(unsafe.Pointer(readerVideoProcessing)), 1); err != nil {
		return err
	}
	if err := callProc(procMFCreateSourceReaderFromSource, uintptr(unsafe.Pointer(c.source)), uintptr(unsafe.Pointer(attrs)), uintptr(unsafe.Pointer(&c.reader))); err != nil {
		return err
	}

	var best *mode
	for i := uint32(0); ; i++ {
		var t *comObject
		if err := c.reader.call(readerGetNativeMediaType, firstVideoStream, uintptr(i), uintptr(unsafe.Pointer(&t))); err != nil {
			if err == mfENoMoreTypes {
				break
			}
			return fmt.Errorf("reading its modes: %w", err)
		}
		m := mode{index: i, mjpeg: getGUID(t, mtSubtype) == *videoFormatMJPG}
		m.width, m.height = getPair(t, mtFrameSize)
		if num, den := getPair(t, mtFrameRate); den > 0 {
			m.fps = float64(num) / float64(den)
		}
		t.release()
		if best == nil || m.better(*best, opts.Width, opts.Height) {
			best = &m
		}
	}
	if best == nil {
		return errors.New("has no video modes")
	}
	var native *comObject
	if err := c.reader.call(readerGetNativeMediaType, firstVideoStream, uintptr(best.index), uintptr(unsafe.Pointer(&native))); err != nil {
		return err
	}
	defer native.release()
	if err := c.reader.call(readerSetCurrentMediaType, firstVideoStream, 0, uintptr(unsafe.Pointer(native))); err != nil {
		return fmt.Errorf("choosing its mode: %w", err)
	}

	var rgb *comObject
	if err := callProc(procMFCreateMediaType, uintptr(unsafe.Pointer(&rgb))); err != nil {
		return err
	}
	defer rgb.release()
	if err := rgb.call(attrSetGUID, uintptr(unsafe.Pointer(mtMajorType)), uintptr(unsafe.Pointer(mediaTypeVideo))); err != nil {
		return err
	}
	if err := rgb.call(attrSetGUID, uintptr(unsafe.Pointer(mtSubtype)), uintptr(unsafe.Pointer(videoFormatRGB32))); err != nil {
		return err
	}
	if err := c.reader.call(readerSetCurrentMediaType, firstVideoStream, 0, uintptr(unsafe.Pointer(rgb))); err != nil {
		return fmt.Errorf("converting to RGB: %w", err)
	}
	var current *comObject
	if err := c.reader.call(readerGetCurrentMediaType, firstVideoStream, uintptr(unsafe.Pointer(&current))); err != nil {
		return err
	}
	c.width, c.height = getPair(current, mtFrameSize)
	var stride uint32
	if current.call(attrGetUINT32, uintptr(unsafe.Pointer(mtDefaultStride)), uintptr(unsafe.Pointer(&stride))) == nil {
		c.stride = int(int32(stride))
	} else {
		c.stride = 4 * c.width
	}
	current.release()

	format := "uncompressed"
	if best.mjpeg {
		format = "MJPEG"
	}
	c.Format = fmt.Sprintf("%s %d×%d at %.0f fps, decoded by Media Foundation", format, c.width, c.height, best.fps)
	c.autoExposure() // undo a lock left from the last time
	return nil
}

// loop delivers the camera's frames until Close; it fails the camera when no frames
// come anymore, e.g. it was unplugged.
func (c *Camera) loop() {
	defer close(c.stopped)
	watchdog := time.AfterFunc(noFrames, func() {
		c.fail(fmt.Errorf("camera %s: no frames for %v (unplugged?)", c.Device.Name, noFrames))
	})
	defer watchdog.Stop()
	_, err := withCOM(func() (struct{}, error) {
		var last time.Time
		for {
			var stream, flags uint32
			var timestamp int64
			var sample *comObject
			if err := c.reader.call(readerReadSample, firstVideoStream, 0, uintptr(unsafe.Pointer(&stream)),
				uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&timestamp)), uintptr(unsafe.Pointer(&sample))); err != nil {
				return struct{}{}, err
			}
			if c.closing.Load() {
				sample.release()
				return struct{}{}, ErrClosed
			}
			if flags&(readerError|readerEndOfStream) != 0 {
				sample.release()
				return struct{}{}, errors.New("stopped delivering frames (unplugged?)")
			}
			if sample == nil {
				continue // e.g. a gap in the stream
			}
			watchdog.Reset(noFrames)
			// Allow some jitter, so a 30 fps camera delivers every third frame at 10 fps.
			now := time.Now()
			if c.interval == 0 || now.Sub(last) >= c.interval*3/4 {
				if img := c.copyFrame(sample); img != nil {
					last = now
					c.deliver(img)
				}
			}
			sample.release()
		}
	})
	c.fail(fmt.Errorf("camera %s: %w", c.Device.Name, err)) // after Close, Read keeps ErrClosed
}

// copyFrame converts an RGB32 sample to RGBA, nil if it can't be read.
func (c *Camera) copyFrame(sample *comObject) *image.RGBA {
	var buffer *comObject
	if sample.call(sampleConvertToContiguousBuffer, uintptr(unsafe.Pointer(&buffer))) != nil {
		return nil
	}
	defer buffer.release()
	var data unsafe.Pointer
	var length uint32
	if buffer.call(bufferLock, uintptr(unsafe.Pointer(&data)), 0, uintptr(unsafe.Pointer(&length))) != nil {
		return nil
	}
	defer buffer.call(bufferUnlock)
	if int(length) < abs(c.stride)*c.height {
		return nil
	}
	scan0 := data // the first row
	if c.stride < 0 {
		scan0 = unsafe.Add(data, -c.stride*(c.height-1))
	}

	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	for y := range c.height {
		from := unsafe.Slice((*byte)(unsafe.Add(scan0, y*c.stride)), 4*c.width)
		to := img.Pix[y*img.Stride:]
		for x := 0; x < len(from); x += 4 {
			to[x], to[x+1], to[x+2], to[x+3] = from[x+2], from[x+1], from[x], 255 // BGRX
		}
	}
	return img
}

// LockExposure would keep the current exposure and white balance, but like on Linux,
// UVC cameras don't tell the values their automatic chose: Windows reports the default
// or the last manual value instead (the development board's cameras: exposure -6, i.e.
// 1/64 s, white balance 4600 K), and locking made the images up to three times darker
// or brighter. Exposure also only comes in powers of two. It returns
// ErrExposureLockUnsupported.
func (c *Camera) LockExposure() error {
	return ErrExposureLockUnsupported
}

// autoExposure lets the camera adjust exposure and white balance continuously. UVC
// cameras keep a lock in their hardware, also for other programs and the next start,
// e.g. from darts-o-cam on macOS, so it is undone when opening and closing.
func (c *Camera) autoExposure() {
	set := func(iid *windows.GUID, property uintptr) {
		ctrl := c.source.queryInterface(iid)
		if ctrl == nil {
			return
		}
		defer ctrl.release()
		var value, flags int32
		if ctrl.call(controlGet, property, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&flags))) == nil && flags&controlFlagsAuto == 0 {
			_ = ctrl.call(controlSet, property, uintptr(value), controlFlagsAuto)
		}
	}
	set(iidCameraControl, cameraControlExposure)
	set(iidVideoProcAmp, videoProcWhiteBalance)
}

func (c *Camera) stop() error {
	if c.stopped == nil {
		return nil // not opened
	}
	_, err := withCOM(func() (struct{}, error) {
		c.autoExposure() // don't leave the camera locked for other programs
		// Neither shutting the source down nor flushing ends a waiting ReadSample.
		c.closing.Store(true)
		select {
		case <-c.stopped:
		case <-time.After(noFrames):
			return struct{}{}, fmt.Errorf("camera %s doesn't stop (unplugged?)", c.Device.Name) // left open
		}
		c.release()
		return struct{}{}, nil
	})
	return err
}

// release frees what Open created; the read loop must have ended.
func (c *Camera) release() {
	c.reader.release()
	if c.source != nil {
		_ = c.source.call(sourceShutdown)
		c.source.release()
	}
	c.reader, c.source = nil, nil
}
