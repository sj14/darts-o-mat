package camera

import (
	"errors"
	"image"
	"testing"
	"time"
)

// TestRead checks the frame handover without a real camera.
func TestRead(t *testing.T) {
	c := newCamera(Device{ID: "test"})
	frame := func(n int) *image.RGBA { return image.NewRGBA(image.Rect(0, 0, n, 1)) }

	// Only the newest of the frames that arrived since the last Read is returned.
	c.deliver(frame(1))
	c.deliver(frame(2))
	if img, err := c.Read(); err != nil || img.Bounds().Dx() != 2 {
		t.Fatalf("Read = %v, %v, want the newest frame", img.Bounds(), err)
	}

	// Read waits for a new frame instead of returning the same one again.
	got := make(chan *image.RGBA)
	go func() {
		img, _ := c.Read()
		got <- img
	}()
	select {
	case <-got:
		t.Fatal("Read returned without a new frame")
	case <-time.After(50 * time.Millisecond):
	}
	c.deliver(frame(3))
	if img := <-got; img.Bounds().Dx() != 3 {
		t.Errorf("Read after waiting = %v, want frame 3", img.Bounds())
	}

	// Close ends a waiting Read and may be called twice.
	go func() {
		time.Sleep(20 * time.Millisecond)
		c.Close()
	}()
	if _, err := c.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("Read after Close: got %v, want ErrClosed", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	c.deliver(frame(4)) // late frames from the camera are ignored
	if _, err := c.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("Read after late frame: got %v, want ErrClosed", err)
	}
}
