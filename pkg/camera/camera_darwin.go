package camera

// AVFoundation through purego: the Objective-C runtime and the C functions of
// CoreMedia/CoreVideo are called directly, so no cgo and no Xcode is needed to build.
// Objects are reference counted by hand (no ARC): what is created with alloc/init is
// released in stop, everything else lives in an autorelease pool around each call.

import (
	"errors"
	"fmt"
	"image"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

type platform struct {
	device, session, input, output, delegate objc.ID
	queue                                    uintptr

	interval  time.Duration // minimum time between delivered frames, 0 for all
	last      time.Time     // when the last frame was delivered, only used on queue
	lastFrame chan struct{} // signals each delivered frame to the watchdog
	done      chan struct{}
}

var (
	loadOnce sync.Once
	loadErr  error

	mediaTypeVideo                                objc.ID
	keyPixelFormat, keyWidth, keyHeight           objc.ID
	presets                                       = map[[2]int]objc.ID{}
	frameDelegate                                 objc.Class
	delegates                                     sync.Map // delegate objc.ID → *Camera
	dispatchQueueCreate                           func(label string, attr uintptr) uintptr
	dispatchRelease                               func(object uintptr)
	dispatchSync                                  func(queue uintptr, block objc.Block)
	cmSampleBufferGetImageBuffer                  func(buffer uintptr) uintptr
	cvPixelBufferLockBaseAddress                  func(buffer uintptr, flags uint64) int32
	cvPixelBufferUnlockBaseAddress                func(buffer uintptr, flags uint64) int32
	cvPixelBufferGetBaseAddress                   func(buffer uintptr) uintptr
	cvPixelBufferGetBytesPerRow                   func(buffer uintptr) uintptr
	cvPixelBufferGetWidth, cvPixelBufferGetHeight func(buffer uintptr) uintptr
	cvPixelBufferGetPixelFormatType               func(buffer uintptr) uint32
	selectors                                     sync.Map // string → objc.SEL
)

const (
	pixelFormatBGRA        = 'B'<<24 | 'G'<<16 | 'R'<<8 | 'A' // kCVPixelFormatType_32BGRA
	lockReadOnly           = 1                                // kCVPixelBufferLock_ReadOnly
	authorized             = 3                                // AVAuthorizationStatusAuthorized
	notDetermined          = 0                                // AVAuthorizationStatusNotDetermined
	exposureLocked         = 0                                // AVCaptureExposureModeLocked
	exposureContinuous     = 2                                // AVCaptureExposureModeContinuousAutoExposure
	whiteBalanceLocked     = 0                                // AVCaptureWhiteBalanceModeLocked
	whiteBalanceContinuous = 2                                // AVCaptureWhiteBalanceModeContinuousAutoWhiteBalance
	frameworks             = "/System/Library/Frameworks/"
	delegateClassName      = "DartsOCamFrameDelegate"
	delegateMethodName     = "captureOutput:didOutputSampleBuffer:fromConnection:"
	privacySettingsHint    = "allow camera access for the terminal in System Settings → Privacy & Security → Camera"
)

func sel(name string) objc.SEL {
	if s, ok := selectors.Load(name); ok {
		return s.(objc.SEL)
	}
	s := objc.RegisterName(name)
	selectors.Store(name, s)
	return s
}

func class(name string) objc.ID {
	return objc.ID(objc.GetClass(name))
}

// pointer turns an address from C into a Go pointer without tripping go vet.
func pointer(addr uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&addr))
}

func load() error {
	loadOnce.Do(func() { loadErr = loadFrameworks() })
	return loadErr
}

func loadFrameworks() error {
	open := func(path string) (uintptr, error) {
		return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if _, err := open(frameworks + "Foundation.framework/Foundation"); err != nil {
		return err
	}
	avf, err := open(frameworks + "AVFoundation.framework/AVFoundation")
	if err != nil {
		return err
	}
	cm, err := open(frameworks + "CoreMedia.framework/CoreMedia")
	if err != nil {
		return err
	}
	cv, err := open(frameworks + "CoreVideo.framework/CoreVideo")
	if err != nil {
		return err
	}
	system, err := open("/usr/lib/libSystem.B.dylib")
	if err != nil {
		return err
	}

	purego.RegisterLibFunc(&dispatchQueueCreate, system, "dispatch_queue_create")
	purego.RegisterLibFunc(&dispatchRelease, system, "dispatch_release")
	purego.RegisterLibFunc(&dispatchSync, system, "dispatch_sync")
	purego.RegisterLibFunc(&cmSampleBufferGetImageBuffer, cm, "CMSampleBufferGetImageBuffer")
	purego.RegisterLibFunc(&cvPixelBufferLockBaseAddress, cv, "CVPixelBufferLockBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferUnlockBaseAddress, cv, "CVPixelBufferUnlockBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferGetBaseAddress, cv, "CVPixelBufferGetBaseAddress")
	purego.RegisterLibFunc(&cvPixelBufferGetBytesPerRow, cv, "CVPixelBufferGetBytesPerRow")
	purego.RegisterLibFunc(&cvPixelBufferGetWidth, cv, "CVPixelBufferGetWidth")
	purego.RegisterLibFunc(&cvPixelBufferGetHeight, cv, "CVPixelBufferGetHeight")
	purego.RegisterLibFunc(&cvPixelBufferGetPixelFormatType, cv, "CVPixelBufferGetPixelFormatType")

	// Constants are global NSString/CFString variables of the frameworks.
	constant := func(lib uintptr, name string) (objc.ID, error) {
		addr, err := purego.Dlsym(lib, name)
		if err != nil {
			return 0, err
		}
		return *(*objc.ID)(pointer(addr)), nil
	}
	if mediaTypeVideo, err = constant(avf, "AVMediaTypeVideo"); err != nil {
		return err
	}
	if keyPixelFormat, err = constant(cv, "kCVPixelBufferPixelFormatTypeKey"); err != nil {
		return err
	}
	if keyWidth, err = constant(cv, "kCVPixelBufferWidthKey"); err != nil {
		return err
	}
	if keyHeight, err = constant(cv, "kCVPixelBufferHeightKey"); err != nil {
		return err
	}
	for _, size := range [][2]int{{320, 240}, {352, 288}, {640, 480}, {960, 540}, {1280, 720}, {1920, 1080}, {3840, 2160}} {
		if p, err := constant(avf, fmt.Sprintf("AVCaptureSessionPreset%dx%d", size[0], size[1])); err == nil {
			presets[size] = p
		}
	}

	frameDelegate, err = objc.RegisterClass(delegateClassName, objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel(delegateMethodName), Fn: didOutputSampleBuffer},
	})
	return err
}

// withPool runs fn in an autorelease pool on a fixed thread, as Objective-C expects.
func withPool[T any](fn func() (T, error)) (T, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := class("NSAutoreleasePool").Send(sel("alloc")).Send(sel("init"))
	defer pool.Send(sel("drain"))
	return fn()
}

func goString(s objc.ID) string {
	if s == 0 {
		return ""
	}
	return objc.Send[string](s, sel("UTF8String"))
}

func nsString(s string) objc.ID {
	return class("NSString").Send(sel("stringWithUTF8String:"), s)
}

func nsErrorText(err objc.ID) string {
	if err == 0 {
		return "unknown error"
	}
	return goString(err.Send(sel("localizedDescription")))
}

// modelPattern finds the USB ids in an AVCaptureDevice's modelID, e.g. "UVC Camera VendorID_7749 ProductID_32802".
var modelPattern = regexp.MustCompile(`VendorID_(\d+) ProductID_(\d+)`)

func deviceInfo(d objc.ID) Device {
	info := Device{ID: goString(d.Send(sel("uniqueID"))), Name: goString(d.Send(sel("localizedName")))}
	if m := modelPattern.FindStringSubmatch(goString(d.Send(sel("modelID")))); m != nil {
		vendor, _ := strconv.Atoi(m[1])
		product, _ := strconv.Atoi(m[2])
		info.Model = fmt.Sprintf("%04x:%04x", vendor, product)
	}
	return info
}

// Devices lists the connected cameras.
func Devices() ([]Device, error) {
	if err := load(); err != nil {
		return nil, err
	}
	return withPool(func() ([]Device, error) {
		list := class("AVCaptureDevice").Send(sel("devicesWithMediaType:"), mediaTypeVideo)
		devices := []Device{}
		for i := range objc.Send[uint](list, sel("count")) {
			devices = append(devices, deviceInfo(list.Send(sel("objectAtIndex:"), i)))
		}
		return devices, nil
	})
}

// authorize asks for camera access the first time; macOS shows a dialog for the terminal.
func authorize() error {
	devices := class("AVCaptureDevice")
	switch objc.Send[int](devices, sel("authorizationStatusForMediaType:"), mediaTypeVideo) {
	case authorized:
		return nil
	case notDetermined:
		granted := make(chan bool, 1)
		block := objc.NewBlock(func(_ objc.Block, ok bool) { granted <- ok })
		defer block.Release()
		devices.Send(sel("requestAccessForMediaType:completionHandler:"), mediaTypeVideo, block)
		if <-granted {
			return nil
		}
	}
	return errors.New("no camera access: " + privacySettingsHint)
}

// Open starts delivering frames of the device with the given ID.
func Open(id string, opts Options) (*Camera, error) {
	if err := load(); err != nil {
		return nil, err
	}
	c, err := withPool(func() (*Camera, error) { return open(id, opts) })
	if err != nil {
		return nil, err
	}
	go c.watch()
	return c, nil
}

func open(id string, opts Options) (*Camera, error) {
	if err := authorize(); err != nil {
		return nil, err
	}
	device := class("AVCaptureDevice").Send(sel("deviceWithUniqueID:"), nsString(id))
	if device == 0 {
		return nil, fmt.Errorf("no camera with id %q", id)
	}
	c := newCamera(deviceInfo(device))
	c.device = device.Send(sel("retain"))
	c.lastFrame, c.done = make(chan struct{}, 1), make(chan struct{})
	if opts.FPS > 0 {
		c.interval = time.Second / time.Duration(opts.FPS)
	}
	fail := func(format string, args ...any) (*Camera, error) {
		c.release()
		return nil, fmt.Errorf("camera %s: "+format, append([]any{c.Device.Name}, args...)...)
	}

	var nsErr objc.ID
	c.input = class("AVCaptureDeviceInput").Send(sel("alloc")).Send(sel("initWithDevice:error:"), device, unsafe.Pointer(&nsErr))
	if c.input == 0 {
		return fail("%s", nsErrorText(nsErr))
	}

	c.session = class("AVCaptureSession").Send(sel("alloc")).Send(sel("init"))
	c.session.Send(sel("beginConfiguration"))
	// The preset picks the camera's own mode; the output below scales if there is none.
	if preset, ok := presets[[2]int{opts.Width, opts.Height}]; ok && objc.Send[bool](c.session, sel("canSetSessionPreset:"), preset) {
		c.session.Send(sel("setSessionPreset:"), preset)
	}
	if !objc.Send[bool](c.session, sel("canAddInput:"), c.input) {
		return fail("can't use it as input")
	}
	c.session.Send(sel("addInput:"), c.input)

	c.output = class("AVCaptureVideoDataOutput").Send(sel("alloc")).Send(sel("init"))
	settings := class("NSMutableDictionary").Send(sel("dictionary"))
	number := class("NSNumber")
	settings.Send(sel("setObject:forKey:"), number.Send(sel("numberWithUnsignedInt:"), uint32(pixelFormatBGRA)), keyPixelFormat)
	if opts.Width > 0 && opts.Height > 0 {
		settings.Send(sel("setObject:forKey:"), number.Send(sel("numberWithInt:"), int32(opts.Width)), keyWidth)
		settings.Send(sel("setObject:forKey:"), number.Send(sel("numberWithInt:"), int32(opts.Height)), keyHeight)
	}
	c.output.Send(sel("setVideoSettings:"), settings)
	c.output.Send(sel("setAlwaysDiscardsLateVideoFrames:"), true)

	c.delegate = objc.ID(frameDelegate).Send(sel("alloc")).Send(sel("init"))
	delegates.Store(c.delegate, c)
	c.queue = dispatchQueueCreate("darts-o-mat.camera", 0) // serial
	c.output.Send(sel("setSampleBufferDelegate:queue:"), c.delegate, c.queue)
	if !objc.Send[bool](c.session, sel("canAddOutput:"), c.output) {
		return fail("can't deliver frames")
	}
	c.session.Send(sel("addOutput:"), c.output)
	c.session.Send(sel("commitConfiguration"))

	c.session.Send(sel("startRunning"))
	if !objc.Send[bool](c.session, sel("isRunning")) {
		return fail("doesn't start (used by another program?)")
	}
	_ = c.autoExposure() // undo a lock left from the last time
	return c, nil
}

// didOutputSampleBuffer is called by AVFoundation on the camera's queue for every frame.
func didOutputSampleBuffer(self objc.ID, _ objc.SEL, _ objc.ID, sampleBuffer uintptr, _ objc.ID) {
	v, ok := delegates.Load(self)
	if !ok {
		return
	}
	c := v.(*Camera)
	now := time.Now()
	// Allow some jitter, so a 30 fps camera delivers every third frame at 10 fps.
	if c.interval > 0 && now.Sub(c.last) < c.interval*3/4 {
		return
	}
	img := copyFrame(sampleBuffer)
	if img == nil {
		return
	}
	c.last = now
	c.deliver(img)
	select {
	case c.lastFrame <- struct{}{}:
	default:
	}
}

// copyFrame converts a BGRA sample buffer to an RGBA image, nil if it holds no such image.
func copyFrame(sampleBuffer uintptr) *image.RGBA {
	buffer := cmSampleBufferGetImageBuffer(sampleBuffer)
	if buffer == 0 || cvPixelBufferGetPixelFormatType(buffer) != pixelFormatBGRA {
		return nil
	}
	if cvPixelBufferLockBaseAddress(buffer, lockReadOnly) != 0 {
		return nil
	}
	defer cvPixelBufferUnlockBaseAddress(buffer, lockReadOnly)

	base := cvPixelBufferGetBaseAddress(buffer)
	w, h := int(cvPixelBufferGetWidth(buffer)), int(cvPixelBufferGetHeight(buffer))
	stride := int(cvPixelBufferGetBytesPerRow(buffer))
	if base == 0 || w == 0 || h == 0 {
		return nil
	}
	src := unsafe.Slice((*byte)(pointer(base)), stride*h)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		from, to := src[y*stride:y*stride+4*w], img.Pix[y*img.Stride:y*img.Stride+4*w]
		for x := 0; x < len(to); x += 4 {
			to[x], to[x+1], to[x+2], to[x+3] = from[x+2], from[x+1], from[x], 255
		}
	}
	return img
}

// watch fails the camera when no frames come anymore, e.g. it was unplugged.
func (c *Camera) watch() {
	timer := time.NewTimer(noFrames)
	defer timer.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-c.lastFrame:
			timer.Reset(noFrames)
		case <-timer.C:
			c.fail(fmt.Errorf("camera %s: no frames for %v (unplugged?)", c.Device.Name, noFrames))
			return
		}
	}
}

// LockExposure keeps the current exposure and white balance, so a hand in front of
// the camera doesn't make it adjust and change the whole image. Call it once the
// exposure has settled. Returns ErrExposureLockUnsupported if the camera can do neither.
// Close lets the camera adjust again.
func (c *Camera) LockExposure() error {
	applied, err := c.setModes(exposureLocked, whiteBalanceLocked)
	if err == nil && !applied {
		err = ErrExposureLockUnsupported
	}
	return err
}

// autoExposure lets the camera adjust exposure and white balance continuously. UVC
// cameras keep a lock in their hardware, also for other programs and the next start,
// so it is undone when opening and closing.
func (c *Camera) autoExposure() error {
	_, err := c.setModes(exposureContinuous, whiteBalanceContinuous)
	return err
}

// setModes sets the exposure and white balance mode where supported and reports
// whether any of them is in effect afterwards.
func (c *Camera) setModes(exposure, whiteBalance int) (bool, error) {
	return withPool(func() (bool, error) {
		d := c.device
		if !objc.Send[bool](d, sel("lockForConfiguration:"), uintptr(0)) {
			return false, fmt.Errorf("camera %s: can't configure it", c.Device.Name)
		}
		defer d.Send(sel("unlockForConfiguration"))
		applied := false
		set := func(name string, mode int) {
			setter, supported := "set"+strings.ToUpper(name[:1])+name[1:]+":", "is"+strings.ToUpper(name[:1])+name[1:]+"Supported:"
			if objc.Send[bool](d, sel("respondsToSelector:"), sel(setter)) && objc.Send[bool](d, sel(supported), mode) {
				d.Send(sel(setter), mode)
				// UVC cameras may accept a mode without applying it, so read it back.
				applied = applied || objc.Send[int](d, sel(name)) == mode
			}
		}
		set("exposureMode", exposure)
		set("whiteBalanceMode", whiteBalance)
		return applied, nil
	})
}

func (c *Camera) stop() error {
	_, err := withPool(func() (struct{}, error) {
		if c.session != 0 {
			_ = c.autoExposure() // don't leave the camera locked for other programs
			c.session.Send(sel("stopRunning"))
		}
		c.release()
		return struct{}{}, nil
	})
	return err
}

// release frees what open created; frames still being delivered are waited for.
func (c *Camera) release() {
	if c.done != nil {
		close(c.done)
	}
	if c.output != 0 {
		c.output.Send(sel("setSampleBufferDelegate:queue:"), objc.ID(0), uintptr(0))
	}
	if c.queue != 0 {
		block := objc.NewBlock(func(objc.Block) {})
		dispatchSync(c.queue, block) // the queue is serial: a running callback has finished
		block.Release()
		dispatchRelease(c.queue)
	}
	if c.delegate != 0 {
		delegates.Delete(c.delegate)
	}
	for _, o := range []objc.ID{c.session, c.output, c.input, c.delegate, c.device} {
		if o != 0 {
			o.Send(sel("release"))
		}
	}
	c.session, c.output, c.input, c.delegate, c.device, c.queue = 0, 0, 0, 0, 0, 0
}
