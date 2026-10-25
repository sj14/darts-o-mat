package camapp

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

// event returns a detection of two cameras with tiny frames.
func event() Event {
	frames := []*image.RGBA{image.NewRGBA(image.Rect(0, 0, 4, 4)), image.NewRGBA(image.Rect(0, 0, 4, 4))}
	return Event{Cameras: []string{"cam1", "cam2"}, Event: &vision.Event{Kind: vision.EventTakeout, Before: frames, After: frames}}
}

func files(t *testing.T, dir string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// The frames of the detections shown are kept as JPEG; nothing is written to disk (a
// Raspberry Pi's SD card).
func TestEventFramesInMemory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	before := files(t, dir)

	for range maxEvents + 2 {
		a.AddEvent(event())
	}
	if got := files(t, dir); len(got) != len(before) {
		t.Errorf("files %v after the detections, want %v", got, before)
	}
	if _, _, ok := a.EventFrame(2, 0, "before"); ok {
		t.Error("frames of a detection no longer shown are still there")
	}
	data, e, ok := a.EventFrame(maxEvents+2, 1, "after")
	if !ok || e.FrameName(1, "after") != "022-cam2-after.jpg" {
		t.Fatalf("newest detection's frame: %v, %q", ok, e.FrameName(1, "after"))
	}
	if img, err := DecodeJPEG(data); err != nil || img.Bounds().Dx() != 4 {
		t.Errorf("frame isn't the JPEG of the detection's frame: %v", err)
	}
	for _, e := range a.Events(0) {
		if e.Before != nil || e.After != nil {
			t.Fatalf("detection %d keeps its frames as images", e.ID)
		}
	}
	for _, bad := range []struct {
		camera int
		kind   string
	}{{2, "after"}, {-1, "after"}, {0, "during"}} {
		if _, _, ok := a.EventFrame(maxEvents+2, bad.camera, bad.kind); ok {
			t.Errorf("frame of camera %d, %q: want none", bad.camera, bad.kind)
		}
	}
}

// With -save, the frames of all detections are written, also kept from earlier starts.
func TestEventFramesSaved(t *testing.T) {
	t.Parallel()
	saveDir := t.TempDir()
	old := filepath.Join(saveDir, "099-cam1-before.jpg")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := New(t.TempDir(), Options{SaveDir: saveDir})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for range maxEvents + 2 {
		a.AddEvent(event())
	}
	if got := len(files(t, saveDir)); got != (maxEvents+2)*2*2+1 {
		t.Errorf("%d frames on disk, want all %d plus the old one", got, (maxEvents+2)*2*2)
	}
}

// The report of a detection is sent at once and may be answered before the detection is
// kept (its frames are still being encoded): the answer is shown all the same, and later
// ones replace it.
func TestReportBeforeEventIsKept(t *testing.T) {
	t.Parallel()
	a, err := New(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	e := event()
	e.ID, e.Sending = a.newEventID(), true
	a.setReport(report{event: e.ID}, 7, nil, false)
	a.AddEvent(e)
	if got, _ := a.Event(e.ID); got.Sending || got.Game != 7 {
		t.Errorf("event %+v, want it entered in game 7", got)
	}
	a.setReport(report{event: e.ID}, 0, errStopped, false)
	if got, _ := a.Event(e.ID); got.Game != 0 || got.ReportErr != errStopped {
		t.Errorf("after another answer: game %d, %v", got.Game, got.ReportErr)
	}
}
