// Package camera reads frames from the cameras connected to this computer, directly
// through the operating system's camera interface (macOS: AVFoundation, Linux: V4L2,
// Windows: Media Foundation) and without cgo.
package camera

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"sync"
	"time"
	"unsafe"
)

// ErrUnsupported means camera access isn't implemented for this operating system yet.
var ErrUnsupported = errors.New("camera access is only implemented for macOS, Linux and Windows")

// ErrExposureLockUnsupported means the camera can neither lock its exposure nor its white balance.
var ErrExposureLockUnsupported = errors.New("the camera can't lock its exposure")

// noFrames is how long a running camera may deliver nothing before Read fails, e.g.
// unplugged. A variable for the tests.
var noFrames = 5 * time.Second

// ErrClosed is returned by Read once the camera was closed.
var ErrClosed = errors.New("camera closed")

// Device is a connected camera.
type Device struct {
	// ID opens the device. It identifies the device while it stays plugged in, but it
	// may change when it is plugged into another port (macOS: the uniqueID, made of the
	// USB location and the model; Windows: the symbolic link, which contains the USB
	// port) or, on Linux, when it is plugged in again or after a restart (the device
	// file, e.g. "/dev/video2").
	ID   string
	Name string // e.g. "Autodarts DIY Cam"; identical cameras share it
	// Model is the USB vendor:product id, e.g. "1e45:8022", empty if unknown.
	Model string
}

// Options for opening a camera.
type Options struct {
	// Width and Height are the wanted frame size: the camera's own mode closest to it
	// is used, frames aren't scaled. So the frames may come in another size.
	Width, Height int
	// FPS limits the frames delivered per second; frames in between are dropped.
	// 0 delivers every frame of the camera.
	FPS int
}

// Camera delivers the frames of an opened device.
type Camera struct {
	Device Device
	// Format tells how the frames arrive, e.g. "MJPEG 1280×720, decoded with
	// libjpeg-turbo"; empty if the operating system doesn't say.
	Format string

	interval time.Duration // minimum time between delivered frames, 0 for all
	last     time.Time     // when the last frame was due, only used by the delivering goroutine

	mu       sync.Mutex
	cond     *sync.Cond
	latest   *image.RGBA
	seq      uint64 // counts the frames delivered
	read     uint64 // seq of the frame last returned by Read
	err      error
	watchdog *time.Timer // fails the camera when no frames come, nil until watch

	closeOnce sync.Once
	closeErr  error

	platform // the operating system's part
}

func newCamera(d Device, opts Options) *Camera {
	c := &Camera{Device: d}
	if opts.FPS > 0 {
		c.interval = time.Second / time.Duration(opts.FPS)
	}
	c.cond = sync.NewCond(&c.mu)
	return c
}

// watch fails the camera when it delivers no frames for noFrames, e.g. it was
// unplugged. The operating system's part calls it once the camera runs.
func (c *Camera) watch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchdog = time.AfterFunc(noFrames, func() {
		c.fail(fmt.Errorf("camera %s: no frames for %v (unplugged?)", c.Device.Name, noFrames))
	})
}

// due reports whether a frame arriving now is to be delivered, or dropped to keep to
// Options.FPS. Only the goroutine delivering the frames calls it, before it copies or
// decodes the frame.
func (c *Camera) due() bool {
	now := time.Now()
	// Allow some jitter, so a 30 fps camera delivers every third frame at 10 fps.
	if c.interval > 0 && now.Sub(c.last) < c.interval*3/4 {
		return false
	}
	c.last = now
	return true
}

// deliver hands a new frame to Read.
func (c *Camera) deliver(img *image.RGBA) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {
		return
	}
	c.latest = img
	c.seq++
	if c.watchdog != nil {
		c.watchdog.Reset(noFrames)
	}
	c.cond.Broadcast()
}

// fail ends delivering frames with err.
func (c *Camera) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
	if c.watchdog != nil {
		c.watchdog.Stop()
	}
	c.cond.Broadcast()
}

// swapBGRA copies a row of BGRA or BGRX pixels to RGBA, opaque. It runs for every frame
// on macOS and Windows, so it works on whole pixels as uint32: 1.4 instead of 2.4 ms per
// 1280×720 frame (Intel i7-9750H). Two pixels as uint64 took 1.25 ms, but need rows
// aligned to 8 bytes; Go's experimental simd package 1.06 ms – the memory is the limit.
func swapBGRA(to, from []byte) {
	if !littleEndian {
		for x := 0; x+3 < len(from); x += 4 {
			to[x], to[x+1], to[x+2], to[x+3] = from[x+2], from[x+1], from[x], 255
		}
		return
	}
	// Pixel rows start at multiples of 4 bytes, so they can be read as uint32.
	src := unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(from))), len(from)/4)
	dst := unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(to))), len(to)/4)[:len(src)]
	for i, p := range src {
		// Little-endian, B G R A in memory is A<<24 | R<<16 | G<<8 | B.
		dst[i] = 0xff000000 | p&0xff00 | p&0xff<<16 | p>>16&0xff
	}
}

// littleEndian is whether a uint32 holds a pixel's bytes in reverse order (true on
// amd64, arm64 and the other systems with camera support).
var littleEndian = binary.NativeEndian.Uint16([]byte{1, 0}) == 1

// Read waits for a frame newer than the one it returned last time. Frames that arrive
// while nobody reads are dropped, only the newest is kept. The image belongs to the
// caller.
func (c *Camera) Read() (*image.RGBA, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for c.seq == c.read && c.err == nil {
		c.cond.Wait()
	}
	if c.err != nil {
		return nil, c.err
	}
	c.read = c.seq
	return c.latest, nil
}

// Close stops the camera; a waiting Read returns ErrClosed. It may be called more than once.
func (c *Camera) Close() error {
	c.closeOnce.Do(func() {
		c.fail(ErrClosed)
		c.closeErr = c.stop()
	})
	return c.closeErr
}
