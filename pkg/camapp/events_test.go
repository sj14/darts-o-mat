package camapp

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/vision"
)

// event returns a detection of two cameras with tiny frames.
func event() Event {
	frames := []*image.RGBA{image.NewRGBA(image.Rect(0, 0, 4, 4)), image.NewRGBA(image.Rect(0, 0, 4, 4))}
	return Event{Cameras: []string{"cam1", "cam2"}, Event: &vision.Event{Kind: vision.EventTakeout, Before: frames, After: frames}}
}

func files(t *testing.T, dir string) []string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func TestEventFramesOnDisk(t *testing.T) {
	frameDir := t.TempDir()
	// A file from an earlier start: the numbers start over, so it goes.
	if err := os.WriteFile(filepath.Join(frameDir, "001-cam1-before.jpg"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := New(t.TempDir(), Options{FrameDir: frameDir})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if got := files(t, frameDir); len(got) != 0 {
		t.Errorf("frames of an earlier start weren't removed: %v", got)
	}

	for range maxEvents + 2 {
		a.AddEvent(event())
	}
	if got := len(files(t, frameDir)); got != maxEvents*2*2 {
		t.Errorf("%d frames on disk, want %d (%d detections, 2 cameras, before and after)", got, maxEvents*2*2, maxEvents)
	}
	if _, ok := a.EventFrame(2, 0, "before"); ok {
		t.Error("frames of a detection no longer shown are still there")
	}
	path, ok := a.EventFrame(maxEvents+2, 1, "after")
	if !ok || filepath.Base(path) != "022-cam2-after.jpg" {
		t.Errorf("newest detection's frame = %q, %v", path, ok)
	}
	for _, e := range a.Events() {
		if e.Before != nil || e.After != nil {
			t.Fatalf("detection %d keeps its frames in memory", e.ID)
		}
	}
	for _, bad := range []struct {
		camera int
		kind   string
	}{{2, "after"}, {-1, "after"}, {0, "during"}} {
		if _, ok := a.EventFrame(maxEvents+2, bad.camera, bad.kind); ok {
			t.Errorf("frame of camera %d, %q: want none", bad.camera, bad.kind)
		}
	}
}

// With -save, the frames of all detections are kept, also from earlier starts.
func TestEventFramesKept(t *testing.T) {
	frameDir := t.TempDir()
	old := filepath.Join(frameDir, "099-cam1-before.jpg")
	if err := os.WriteFile(old, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := New(t.TempDir(), Options{FrameDir: frameDir, KeepFrames: true})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	for range maxEvents + 2 {
		a.AddEvent(event())
	}
	if got := len(files(t, frameDir)); got != (maxEvents+2)*2*2+1 {
		t.Errorf("%d frames on disk, want all %d plus the old one", got, (maxEvents+2)*2*2)
	}
}
