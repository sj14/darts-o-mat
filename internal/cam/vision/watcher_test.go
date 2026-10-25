package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

func TestWatcher(t *testing.T) {
	t.Parallel()
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, err := calib.Camera("cam3")
	if err != nil {
		t.Fatal(err)
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}

	empty := loadRGBA(t, "testdata/cam3.jpg")
	withDart := loadRGBA(t, "testdata/cam3.jpg")
	tip := single1(t, toBoard)
	drawDart(withDart, tip, dartboard.Point{X: 0.1, Y: -1})

	w := NewWatcher([]Mapping{{ToBoard: toBoard}}, DefaultWatchOptions)
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
	if e.Kind != EventDart || e.Dart.Number != 1 || !e.Dart.Segment.IsSingle() {
		t.Errorf("event = %+v, want a single 1", e)
	}
	want := toBoard.Apply(tip)
	if dist := math.Hypot(e.Board.X-want.X, e.Board.Y-want.Y); dist > 3 {
		t.Errorf("dart at %.1f,%.1f mm, want %.1f,%.1f (off by %.1f mm)", e.Board.X, e.Board.Y, want.X, want.Y, dist)
	}
}

func TestWatcherIgnoresStillBoard(t *testing.T) {
	t.Parallel()
	img := loadRGBA(t, "testdata/cam3.jpg")
	w := NewWatcher([]Mapping{{ToBoard: Homography{1, 0, 0, 0, 1, 0, 0, 0, 1}}}, DefaultWatchOptions)
	for range 10 {
		if e := w.Feed([]*image.RGBA{img}); e != nil {
			t.Fatalf("unexpected event %+v", e)
		}
	}
}

// The caller may reuse the slice of frames between calls, like darts-o-cam does.
func TestWatcherReusedSlice(t *testing.T) {
	t.Parallel()
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam3")
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}

	empty := loadRGBA(t, "testdata/cam3.jpg")
	withDart := loadRGBA(t, "testdata/cam3.jpg")
	drawDart(withDart, single1(t, toBoard), dartboard.Point{X: 0.1, Y: -1})

	w := NewWatcher([]Mapping{{ToBoard: toBoard}}, DefaultWatchOptions)
	frames := make([]*image.RGBA, 1)
	var events int
	for _, f := range []*image.RGBA{empty, empty, empty, withDart, withDart, withDart, withDart} {
		frames[0] = f
		if w.Feed(frames) != nil {
			events++
		}
	}
	if events != 1 {
		t.Errorf("got %d events, want 1", events)
	}
}

// single1 returns the image position of the middle of the inner single 1.
func single1(t *testing.T, toBoard Homography) dartboard.Point {
	t.Helper()
	toImage, err := toBoard.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	return toImage.Apply(dartboard.Polar(60, dartboard.SegmentAngle))
}

// Pulling a dart out changes the image as much as throwing it, but must not count as a dart.
func TestWatcherTakeout(t *testing.T) {
	t.Parallel()
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam3")
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}

	empty := loadRGBA(t, "testdata/cam3.jpg")
	one := loadRGBA(t, "testdata/cam3.jpg")
	tip1 := single1(t, toBoard)
	drawDart(one, tip1, dartboard.Point{X: 0.1, Y: -1})
	two := image.NewRGBA(one.Bounds())
	copy(two.Pix, one.Pix)
	toImage, _ := toBoard.Inverse()
	drawDart(two, toImage.Apply(dartboard.Polar(130, 6*dartboard.SegmentAngle)), dartboard.Point{X: -0.2, Y: -1})

	w := NewWatcher([]Mapping{{ToBoard: toBoard}}, DefaultWatchOptions)
	var kinds []EventKind
	for _, f := range []*image.RGBA{empty, empty, one, one, one, one, two, two, two, two, empty, empty, empty, empty} {
		if e := w.Feed([]*image.RGBA{f}); e != nil {
			kinds = append(kinds, e.Kind)
		}
	}
	want := []EventKind{EventDart, EventDart, EventTakeout}
	if !slices.Equal(kinds, want) {
		t.Errorf("events = %v, want %v", kinds, want)
	}

	// After the takeout, a dart at the same place counts again.
	for _, f := range []*image.RGBA{one, one, one, one} {
		if e := w.Feed([]*image.RGBA{f}); e != nil {
			kinds = append(kinds, e.Kind)
		}
	}
	if want = append(want, EventDart); !slices.Equal(kinds, want) {
		t.Errorf("events = %v, want %v", kinds, want)
	}
}

// Darts already in the board when the watcher starts are unknown to it. With an image
// of the empty board it still recognizes pulling them out as a takeout.
func TestWatcherTakeoutOfUnknownDarts(t *testing.T) {
	t.Parallel()
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam3")
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}

	empty := loadRGBA(t, "testdata/cam3-empty.jpg")
	withDart := loadRGBA(t, "testdata/cam3-empty.jpg")
	drawDart(withDart, single1(t, toBoard), dartboard.Point{X: 0.1, Y: -1})

	w := NewWatcher([]Mapping{{ToBoard: toBoard}}, DefaultWatchOptions)
	w.Empty = []*image.RGBA{empty}
	var kinds []EventKind
	// The dart is in the board from the start, then pulled out, then thrown again.
	for _, f := range []*image.RGBA{withDart, withDart, empty, empty, empty, empty, withDart, withDart, withDart, withDart} {
		if e := w.Feed([]*image.RGBA{f}); e != nil {
			kinds = append(kinds, e.Kind)
		}
	}
	if want := []EventKind{EventTakeout, EventDart}; !slices.Equal(kinds, want) {
		t.Errorf("events = %v, want %v", kinds, want)
	}
}

// TestWatcherRemovedReal tells darts thrown from darts pulled out on real recordings,
// by comparing with the empty board.
func TestWatcherRemovedReal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		event, camera string
		removed       bool
	}{
		{"003", "cam3", false},
		{"004", "cam1", false},
		{"005", "cam2", true}, // recorded while pulling the dart out
	}
	for _, tt := range tests {
		t.Run(tt.event+"-"+tt.camera, func(t *testing.T) {
			before := loadRGBA(t, "testdata/throws/"+tt.event+"-"+tt.camera+"-before.jpg")
			after := loadRGBA(t, "testdata/throws/"+tt.event+"-"+tt.camera+"-after.jpg")
			w := NewWatcher(nil, DefaultWatchOptions)
			w.Empty = []*image.RGBA{loadRGBA(t, "testdata/"+tt.camera+"-empty.jpg")}
			changed := Changed(before, after, DefaultDetectOptions, ReferenceScale)
			if got := w.removed([]*image.RGBA{before}, []*image.RGBA{after}, []*Mask{changed}); got != tt.removed {
				t.Errorf("removed = %v, want %v", got, tt.removed)
			}
		})
	}
}

// Three darts pulled out (2026-10-25, event 9, cam2): the dart-shaped part of the change
// is a sliver of 243 px that looked like a throw (ratio 1.85); all changed pixels show the
// takeout. Its own empty board, as calibrated then.
func TestWatcherRemovedAllChangedPixels(t *testing.T) {
	t.Parallel()
	before := loadRGBA(t, "testdata/throws/009-cam2-before.jpg")
	after := loadRGBA(t, "testdata/throws/009-cam2-after.jpg")
	w := NewWatcher(nil, DefaultWatchOptions)
	w.Empty = []*image.RGBA{loadRGBA(t, "testdata/throws/009-cam2-empty.jpg")}
	changed := Changed(before, after, DefaultDetectOptions, ReferenceScale)
	if !w.removed([]*image.RGBA{before}, []*image.RGBA{after}, []*Mask{changed}) {
		t.Error("darts pulled out counted as a throw")
	}
}

// TestWatcherRemovedExposure: while the darts were pulled out, one camera adjusted its
// exposure, so its whole image changed. The other two cameras clearly saw the takeout.
func TestWatcherRemovedExposure(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long; runs without -short, e.g. in CI")
	}
	for _, tt := range []struct {
		event   string
		removed bool
	}{
		{"takeout", true},
		{"throw", false},
	} {
		t.Run(tt.event, func(t *testing.T) {
			var before, after, empty []*image.RGBA
			var changes []*Mask
			for c := 1; c <= 3; c++ {
				name := fmt.Sprintf("testdata/exposure/%s-cam%d-", tt.event, c)
				b, a := loadRGBA(t, name+"before.jpg"), loadRGBA(t, name+"after.jpg")
				before, after = append(before, b), append(after, a)
				empty = append(empty, loadRGBA(t, fmt.Sprintf("testdata/exposure/cam%d-empty.jpg", c)))
				changes = append(changes, Changed(b, a, DefaultDetectOptions, ReferenceScale))
			}
			w := NewWatcher(nil, DefaultWatchOptions)
			w.Empty = empty
			if got := w.removed(before, after, changes); got != tt.removed {
				t.Errorf("removed = %v, want %v", got, tt.removed)
			}
		})
	}
}

// With two cameras (2026-10-25, events 39–40, Linux): during the takeout cam3 adjusted
// its exposure and voted for a throw, cam1 saw the takeout; the tie must not be decided
// by cam3's far larger changed area. The throw after it stays a throw.
func TestWatcherRemovedExposureTwoCameras(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long; runs without -short, e.g. in CI")
	}
	for _, tt := range []struct {
		event   string
		removed bool
	}{
		{"039", true},  // the takeout
		{"040", false}, // the throw after it
	} {
		t.Run(tt.event, func(t *testing.T) {
			var before, after, empty []*image.RGBA
			var changes []*Mask
			for _, c := range []string{"cam1", "cam3"} {
				name := "testdata/throws/" + tt.event + "-" + c + "-"
				b, a := loadRGBA(t, name+"before.jpg"), loadRGBA(t, name+"after.jpg")
				before, after = append(before, b), append(after, a)
				empty = append(empty, loadRGBA(t, "testdata/exposure/two-"+c+"-empty.jpg"))
				changes = append(changes, Changed(b, a, DefaultDetectOptions, ReferenceScale))
			}
			w := NewWatcher(nil, DefaultWatchOptions)
			w.Empty = empty
			if got := w.removed(before, after, changes); got != tt.removed {
				t.Errorf("removed = %v, want %v", got, tt.removed)
			}
		})
	}
}

// A change no camera finds a dart in, that isn't darts pulled out either (told by the
// empty board), is a miss without a position: e.g. a dart far out in the surround,
// where the cameras took a thin change along the board's rim for it (2026-10-13).
func TestWatcherUnlocatedMiss(t *testing.T) {
	t.Parallel()
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam3")
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}
	empty := loadRGBA(t, "testdata/cam3-empty.jpg")
	// Something round in the image's corner, like a flight seen from behind: no dart shape.
	blob := image.NewRGBA(empty.Bounds())
	copy(blob.Pix, empty.Pix)
	for y := 620; y < 680; y++ {
		for x := 60; x < 120; x++ {
			blob.SetRGBA(x, y, color.RGBA{30, 60, 170, 255})
		}
	}

	w := NewWatcher([]Mapping{{ToBoard: toBoard}}, DefaultWatchOptions)
	w.Empty = []*image.RGBA{empty}
	var events []*Event
	for _, f := range []*image.RGBA{empty, empty, blob, blob, blob, blob} {
		if e := w.Feed([]*image.RGBA{f}); e != nil {
			events = append(events, e)
		}
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	if e := events[0]; e.Kind != EventDart || e.Dart.Segment != dartboard.Miss || e.Located {
		t.Errorf("got %s %+v, located %v; want a miss without position", e.Kind, e.Dart, e.Located)
	}
}
