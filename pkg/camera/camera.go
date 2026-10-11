// Package camera reads frames from the cameras connected to this computer, directly
// through the operating system's camera interface (macOS: AVFoundation) and without cgo.
// Linux (V4L2) and Windows (Media Foundation) are not implemented yet.
package camera

import (
	"errors"
	"image"
	"sync"
)

// ErrUnsupported means camera access isn't implemented for this operating system yet.
var ErrUnsupported = errors.New("camera access is only implemented for macOS yet")

// ErrClosed is returned by Read once the camera was closed.
var ErrClosed = errors.New("camera closed")

// Device is a connected camera.
type Device struct {
	// ID opens the device. It identifies the device while it stays plugged in, but it
	// may change when it is plugged into another port (macOS: the uniqueID, made of the
	// USB location and the model).
	ID   string
	Name string // e.g. "Autodarts DIY Cam"; identical cameras share it
	// Model is the USB vendor:product id, e.g. "1e45:8022", empty if unknown.
	Model string
}

// Options for opening a camera.
type Options struct {
	Width, Height int
	// FPS limits the frames delivered per second; frames in between are dropped.
	// 0 delivers every frame of the camera.
	FPS int
}

// Camera delivers the frames of an opened device.
type Camera struct {
	Device Device

	mu     sync.Mutex
	cond   *sync.Cond
	latest *image.RGBA
	seq    uint64 // counts the frames delivered
	read   uint64 // seq of the frame last returned by Read
	err    error

	closeOnce sync.Once
	closeErr  error

	platform // the operating system's part
}

func newCamera(d Device) *Camera {
	c := &Camera{Device: d}
	c.cond = sync.NewCond(&c.mu)
	return c
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
	c.cond.Broadcast()
}

// fail ends delivering frames with err.
func (c *Camera) fail(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
	c.cond.Broadcast()
}

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
