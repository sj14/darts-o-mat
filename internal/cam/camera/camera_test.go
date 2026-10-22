package camera

import (
	"bytes"
	"errors"
	"image"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestRead checks the frame handover without a real camera.
func TestRead(t *testing.T) {
	t.Parallel()
	c := newCamera(Device{ID: "test"}, Options{})
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

// TestDue checks the frame rate limit: a 30 fps camera delivers every third frame at 10 fps.
func TestDue(t *testing.T) {
	t.Parallel()
	if c := newCamera(Device{}, Options{}); !c.due() || !c.due() {
		t.Error("without FPS every frame should be due")
	}

	c := newCamera(Device{}, Options{FPS: 10})
	var due []int
	var lastDue time.Duration // when the last due frame arrived, in camera time
	for i := range 10 {
		arrival := time.Duration(i) * time.Second / 30
		if i > 0 { // the first frame is always due
			c.last = time.Now().Add(lastDue - arrival) // as if the frame arrived just now
		}
		if c.due() {
			due = append(due, i)
			lastDue = arrival
		}
	}
	if want := []int{0, 3, 6, 9}; !slices.Equal(due, want) {
		t.Errorf("due frames = %v, want %v", due, want)
	}
}

// TestWatchdog checks that a camera without frames fails, but not one that delivers.
func TestWatchdog(t *testing.T) {
	// Not parallel: it changes noFrames, which the package's other tests read.
	defer func(d time.Duration) { noFrames = d }(noFrames)
	noFrames = 50 * time.Millisecond

	c := newCamera(Device{Name: "test"}, Options{})
	c.watch()
	for range 5 {
		time.Sleep(noFrames / 2)
		c.deliver(image.NewRGBA(image.Rect(0, 0, 1, 1)))
		if _, err := c.Read(); err != nil {
			t.Fatalf("Read while frames come: %v", err)
		}
	}
	if _, err := c.Read(); err == nil || !strings.Contains(err.Error(), "no frames") {
		t.Errorf("Read without frames: got %v, want the watchdog's error", err)
	}

	// Close stops the watchdog: Read keeps returning ErrClosed.
	c = newCamera(Device{Name: "test"}, Options{})
	c.watch()
	c.Close()
	time.Sleep(2 * noFrames)
	if _, err := c.Read(); !errors.Is(err, ErrClosed) {
		t.Errorf("Read after Close: got %v, want ErrClosed", err)
	}
}

func TestSwapBGRA(t *testing.T) {
	t.Parallel()
	to := make([]byte, 8)
	swapBGRA(to, []byte{1, 2, 3, 0, 4, 5, 6, 0})
	if want := []byte{3, 2, 1, 255, 6, 5, 4, 255}; !bytes.Equal(to, want) {
		t.Errorf("swapBGRA = %v, want %v", to, want)
	}

	// A row of an odd width, into a longer row of an image (only its start is written).
	from := make([]byte, 4*1279)
	for i := range from {
		from[i] = byte(i * 7)
	}
	to = bytes.Repeat([]byte{9}, 4*1280)
	swapBGRA(to, from)
	for x := 0; x < len(from); x += 4 {
		if want := []byte{from[x+2], from[x+1], from[x], 255}; !bytes.Equal(to[x:x+4], want) {
			t.Fatalf("pixel %d = %v, want %v", x/4, to[x:x+4], want)
		}
	}
	if !bytes.Equal(to[len(from):], []byte{9, 9, 9, 9}) {
		t.Errorf("wrote beyond the row: %v", to[len(from):])
	}
}

func BenchmarkSwapBGRA(b *testing.B) {
	const w, h = 1280, 720
	from, to := make([]byte, 4*w*h), make([]byte, 4*w*h)
	for b.Loop() {
		for y := range h {
			swapBGRA(to[4*w*y:], from[4*w*y:4*w*(y+1)])
		}
	}
}
