package vision

import (
	"image"
	"math"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func TestWatcher(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, err := findCamera(calib, "cam3")
	if err != nil {
		t.Fatal(err)
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}

	empty := loadRGBA(t, "testdata/cam3.jpg")
	withDart := loadRGBA(t, "testdata/cam3.jpg")
	// The tip of the real single 1 in this camera (see TestRealDarts).
	tip := dartboard.Point{X: 721, Y: 255}
	drawDart(withDart, tip, dartboard.Point{X: 0.1, Y: -1})

	w := NewWatcher([]Homography{toBoard}, DefaultWatchOptions)
	frames := []*image.RGBA{empty, empty, empty, withDart, withDart, withDart, withDart}
	var events []*Event
	for _, f := range frames {
		if e := w.Feed([]*image.RGBA{f}); e != nil {
			events = append(events, e)
		}
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	e := events[0]
	if e.Kind != EventDart || e.Dart != (throw.Dart{Number: 1, Multiplier: 1}) {
		t.Errorf("event = %+v, want a single 1", e)
	}
	want := toBoard.Apply(tip)
	if dist := math.Hypot(e.Board.X-want.X, e.Board.Y-want.Y); dist > 3 {
		t.Errorf("dart at %.1f,%.1f mm, want %.1f,%.1f (off by %.1f mm)", e.Board.X, e.Board.Y, want.X, want.Y, dist)
	}
}

func TestWatcherIgnoresStillBoard(t *testing.T) {
	img := loadRGBA(t, "testdata/cam3.jpg")
	w := NewWatcher([]Homography{{1, 0, 0, 0, 1, 0, 0, 0, 1}}, DefaultWatchOptions)
	for range 10 {
		if e := w.Feed([]*image.RGBA{img}); e != nil {
			t.Fatalf("unexpected event %+v", e)
		}
	}
}

func findCamera(c Calibration, name string) (CameraCalibration, error) {
	for _, cam := range c.Cameras {
		if cam.Name == name {
			return cam, nil
		}
	}
	return CameraCalibration{}, ErrNoDart
}
