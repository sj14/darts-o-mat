package camera

// V4L2 through ioctl: frames come in memory-mapped buffers of the driver, as MJPEG
// (needed for several cameras on one USB 2.0 port) or, if the camera can't, as YUYV.
// A goroutine per camera waits for frames with poll, an eventfd ends it on Close.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type platform struct {
	fd, wake  int // the device and an eventfd that ends the read loop
	buffers   [][]byte
	streaming bool
	format    pixFormat
	interval  time.Duration // minimum time between delivered frames, 0 for all
	stopped   chan struct{} // closed when the read loop has ended
}

// Structs and constants of linux/videodev2.h.

type capability struct {
	Driver     [16]byte
	Card       [32]byte
	BusInfo    [32]byte
	Version    uint32
	Caps       uint32
	DeviceCaps uint32
	Reserved   [3]uint32
}

type pixFormat struct {
	Width, Height, PixelFormat, Field, BytesPerLine, SizeImage, Colorspace, Priv uint32
	Flags, YCbCrEnc, Quantization, XferFunc                                      uint32
}

type format struct {
	Type uint32
	// The union holds pixFormat; it is pointer-aligned because of v4l2_window.
	Fmt [200 / unsafe.Sizeof(uintptr(0))]uintptr
}

func (f *format) pix() *pixFormat {
	return (*pixFormat)(unsafe.Pointer(&f.Fmt))
}

type requestBuffers struct {
	Count, Type, Memory, Capabilities uint32
	Flags                             uint8
	Reserved                          [3]uint8
}

type buffer struct {
	Index, Type, BytesUsed, Flags, Field uint32
	Timestamp                            unix.Timeval
	Timecode                             [4]uint32
	Sequence, Memory                     uint32
	M                                    uintptr // union; for MMAP the offset as uint32
	Length, Reserved2, RequestFD         uint32
}

func (b *buffer) offset() uint32 {
	return *(*uint32)(unsafe.Pointer(&b.M))
}

type control struct {
	ID    uint32
	Value int32
}

func ioc(dir, nr, size uintptr) uintptr {
	return dir<<30 | size<<16 | 'V'<<8 | nr
}

const iocWrite, iocRead = 1, 2

var (
	vidiocQueryCap  = ioc(iocRead, 0, unsafe.Sizeof(capability{}))
	vidiocSFmt      = ioc(iocRead|iocWrite, 5, unsafe.Sizeof(format{}))
	vidiocReqBufs   = ioc(iocRead|iocWrite, 8, unsafe.Sizeof(requestBuffers{}))
	vidiocQueryBuf  = ioc(iocRead|iocWrite, 9, unsafe.Sizeof(buffer{}))
	vidiocQBuf      = ioc(iocRead|iocWrite, 15, unsafe.Sizeof(buffer{}))
	vidiocDQBuf     = ioc(iocRead|iocWrite, 17, unsafe.Sizeof(buffer{}))
	vidiocStreamOn  = ioc(iocWrite, 18, unsafe.Sizeof(int32(0)))
	vidiocStreamOff = ioc(iocWrite, 19, unsafe.Sizeof(int32(0)))
	vidiocSCtrl     = ioc(iocRead|iocWrite, 28, unsafe.Sizeof(control{}))
)

const (
	capVideoCapture = 0x00000001
	capStreaming    = 0x04000000
	capDeviceCaps   = 0x80000000
	bufTypeCapture  = 1
	memoryMMap      = 1
	fieldNone       = 1
	numBuffers      = 4

	cidAutoWhiteBalance      = 0x0098090c
	cidExposureAuto          = 0x009a0901
	exposureAuto             = 0 // V4L2_EXPOSURE_AUTO
	exposureAperturePriority = 3 // V4L2_EXPOSURE_APERTURE_PRIORITY, the auto mode of UVC cameras
)

func fourcc(s string) uint32 {
	return uint32(s[0]) | uint32(s[1])<<8 | uint32(s[2])<<16 | uint32(s[3])<<24
}

var pixelFormatMJPEG, pixelFormatYUYV = fourcc("MJPG"), fourcc("YUYV")

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
		if errno != unix.EINTR {
			if errno != 0 {
				return errno
			}
			return nil
		}
	}
}

func cString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// queryCap returns the capabilities of the device itself, not of the whole driver.
func queryCap(fd int) (capability, uint32, error) {
	var c capability
	if err := ioctl(fd, vidiocQueryCap, unsafe.Pointer(&c)); err != nil {
		return c, 0, err
	}
	if c.Caps&capDeviceCaps != 0 {
		return c, c.DeviceCaps, nil
	}
	return c, c.Caps, nil
}

// deviceInfo describes /dev/videoN. The name and model come from the USB device,
// like on macOS; V4L2 cuts the card name off at 31 bytes.
func deviceInfo(path string, c capability) Device {
	d := Device{ID: path, Name: cString(c.Card[:])}
	iface, err := filepath.EvalSymlinks(filepath.Join("/sys/class/video4linux", filepath.Base(path), "device"))
	if err != nil {
		return d
	}
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(filepath.Dir(iface), name)) // the USB device of the interface
		return strings.TrimSpace(string(b))
	}
	if product := read("product"); product != "" {
		d.Name = product
	}
	if vendor, product := read("idVendor"), read("idProduct"); vendor != "" && product != "" {
		d.Model = vendor + ":" + product
	}
	return d
}

// Devices lists the connected cameras. UVC cameras have a second video device for
// metadata; only devices that capture video are listed.
func Devices() ([]Device, error) {
	paths, err := filepath.Glob("/dev/video*")
	if err != nil {
		return nil, err
	}
	number := func(path string) int {
		n, _ := strconv.Atoi(strings.TrimPrefix(path, "/dev/video"))
		return n
	}
	slices.SortFunc(paths, func(a, b string) int { return number(a) - number(b) })

	devices := []Device{}
	for _, path := range paths {
		fd, err := unix.Open(path, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			continue // e.g. no permission; Open reports it
		}
		c, caps, err := queryCap(fd)
		unix.Close(fd)
		if err == nil && caps&capVideoCapture != 0 && caps&capStreaming != 0 {
			devices = append(devices, deviceInfo(path, c))
		}
	}
	return devices, nil
}

// Open starts delivering frames of the device with the given ID, e.g. "/dev/video2".
func Open(id string, opts Options) (*Camera, error) {
	fd, err := unix.Open(id, unix.O_RDWR|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		if errors.Is(err, unix.EACCES) {
			return nil, fmt.Errorf("camera %s: %w (is the user in the group video?)", id, err)
		}
		return nil, fmt.Errorf("camera %s: %w", id, err)
	}
	info, caps, err := queryCap(fd)
	if err != nil || caps&capVideoCapture == 0 || caps&capStreaming == 0 {
		unix.Close(fd)
		return nil, fmt.Errorf("camera %s: not a video capture device", id)
	}
	c := newCamera(deviceInfo(id, info))
	c.fd, c.wake = fd, -1
	if opts.FPS > 0 {
		c.interval = time.Second / time.Duration(opts.FPS)
	}
	if err := c.start(opts); err != nil {
		c.release()
		return nil, fmt.Errorf("camera %s (%s): %w", c.Device.Name, id, err)
	}
	c.stopped = make(chan struct{})
	go c.loop()
	return c, nil
}

func (c *Camera) start(opts Options) error {
	// The driver picks the mode closest to the requested size.
	var err error
	for _, pf := range []uint32{pixelFormatMJPEG, pixelFormatYUYV} {
		f := format{Type: bufTypeCapture}
		*f.pix() = pixFormat{Width: uint32(opts.Width), Height: uint32(opts.Height), PixelFormat: pf, Field: fieldNone}
		if err = ioctl(c.fd, vidiocSFmt, unsafe.Pointer(&f)); err == nil && f.pix().PixelFormat == pf {
			c.format = *f.pix()
			break
		}
	}
	if c.format.PixelFormat == 0 {
		return fmt.Errorf("delivers neither MJPEG nor YUYV: %v", err)
	}

	req := requestBuffers{Count: numBuffers, Type: bufTypeCapture, Memory: memoryMMap}
	if err := ioctl(c.fd, vidiocReqBufs, unsafe.Pointer(&req)); err != nil {
		return fmt.Errorf("requesting buffers: %w", err)
	}
	for i := range req.Count {
		b := buffer{Index: i, Type: bufTypeCapture, Memory: memoryMMap}
		if err := ioctl(c.fd, vidiocQueryBuf, unsafe.Pointer(&b)); err != nil {
			return fmt.Errorf("querying buffer: %w", err)
		}
		data, err := unix.Mmap(c.fd, int64(b.offset()), int(b.Length), unix.PROT_READ, unix.MAP_SHARED)
		if err != nil {
			return fmt.Errorf("mapping buffer: %w", err)
		}
		c.buffers = append(c.buffers, data)
		if err := ioctl(c.fd, vidiocQBuf, unsafe.Pointer(&b)); err != nil {
			return fmt.Errorf("queueing buffer: %w", err)
		}
	}

	if c.wake, err = unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK); err != nil {
		return err
	}
	typ := int32(bufTypeCapture)
	if err := ioctl(c.fd, vidiocStreamOn, unsafe.Pointer(&typ)); err != nil {
		return fmt.Errorf("starting (used by another program?): %w", err)
	}
	c.streaming = true
	c.autoExposure() // undo a lock left from the last time
	return nil
}

// loop delivers the camera's frames until Close; it fails the camera when no frames
// come anymore, e.g. it was unplugged.
func (c *Camera) loop() {
	defer close(c.stopped)
	var last time.Time
	for {
		fds := []unix.PollFd{{Fd: int32(c.fd), Events: unix.POLLIN}, {Fd: int32(c.wake), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int(noFrames.Milliseconds()))
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			c.fail(fmt.Errorf("camera %s: %w", c.Device.Name, err))
			return
		case fds[1].Revents != 0:
			return // closed
		case n == 0:
			c.fail(fmt.Errorf("camera %s: no frames for %v (unplugged?)", c.Device.Name, noFrames))
			return
		}

		b := buffer{Type: bufTypeCapture, Memory: memoryMMap}
		if err := ioctl(c.fd, vidiocDQBuf, unsafe.Pointer(&b)); err != nil {
			if errors.Is(err, unix.EAGAIN) {
				continue
			}
			c.fail(fmt.Errorf("camera %s: %w (unplugged?)", c.Device.Name, err))
			return
		}
		// Allow some jitter, so a 30 fps camera delivers every third frame at 10 fps.
		now := time.Now()
		var data []byte
		if c.interval == 0 || now.Sub(last) >= c.interval*3/4 {
			data = bytes.Clone(c.buffers[b.Index][:b.BytesUsed])
		}
		if err := ioctl(c.fd, vidiocQBuf, unsafe.Pointer(&b)); err != nil {
			c.fail(fmt.Errorf("camera %s: %w", c.Device.Name, err))
			return
		}
		if data == nil {
			continue
		}
		if img := c.decode(data); img != nil { // cameras send a broken frame now and then
			last = now
			c.deliver(img)
		}
	}
}

// decode converts a frame to RGBA, nil if it is broken.
func (c *Camera) decode(data []byte) *image.RGBA {
	if c.format.PixelFormat == pixelFormatYUYV {
		return yuyvToRGBA(data, int(c.format.Width), int(c.format.Height), int(c.format.BytesPerLine))
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	return img
}

// yuyvToRGBA converts YUYV 4:2:2: two pixels share their colour in Y0 U Y1 V.
func yuyvToRGBA(data []byte, w, h, stride int) *image.RGBA {
	if stride < 2*w || len(data) < stride*(h-1)+2*w {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		from, to := data[y*stride:y*stride+2*w], img.Pix[y*img.Stride:]
		for x := 0; x+3 < len(from); x += 4 {
			u, v := from[x+1], from[x+3]
			r, g, b := color.YCbCrToRGB(from[x], u, v)
			to[2*x], to[2*x+1], to[2*x+2], to[2*x+3] = r, g, b, 255
			r, g, b = color.YCbCrToRGB(from[x+2], u, v)
			to[2*x+4], to[2*x+5], to[2*x+6], to[2*x+7] = r, g, b, 255
		}
	}
	return img
}

func (c *Camera) setControl(id uint32, value int32) error {
	ctrl := control{ID: id, Value: value}
	return ioctl(c.fd, vidiocSCtrl, unsafe.Pointer(&ctrl))
}

// LockExposure would keep the current exposure and white balance, but UVC cameras
// don't tell V4L2 the values their automatic chose: it reports the default instead
// (the development board's cameras: 166 while their automatic used ~290), so switching
// to manual made the images 40–60 % darker. It returns ErrExposureLockUnsupported.
func (c *Camera) LockExposure() error {
	return ErrExposureLockUnsupported
}

// autoExposure lets the camera adjust exposure and white balance continuously. UVC
// cameras keep a lock in their hardware, also for other programs and the next start,
// e.g. from darts-o-cam on macOS, so it is undone when opening.
func (c *Camera) autoExposure() {
	if c.setControl(cidExposureAuto, exposureAperturePriority) != nil {
		_ = c.setControl(cidExposureAuto, exposureAuto)
	}
	_ = c.setControl(cidAutoWhiteBalance, 1)
}

func (c *Camera) stop() error {
	if c.stopped == nil {
		return nil // not opened
	}
	_, _ = unix.Write(c.wake, binary.NativeEndian.AppendUint64(nil, 1))
	<-c.stopped
	return c.release()
}

// release frees what Open created; the read loop must have ended.
func (c *Camera) release() error {
	if c.streaming {
		typ := int32(bufTypeCapture)
		_ = ioctl(c.fd, vidiocStreamOff, unsafe.Pointer(&typ))
	}
	if c.wake >= 0 {
		unix.Close(c.wake)
	}
	for _, b := range c.buffers {
		_ = unix.Munmap(b)
	}
	c.buffers = nil
	return unix.Close(c.fd)
}
