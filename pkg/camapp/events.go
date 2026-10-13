package camapp

import (
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/vision"
)

// maxEvents is how many detections are shown. Their frames are on disk (about 1.2 MB
// per detection with three cameras at 1280×720), only the rest is kept in memory: in
// memory the frames took about 22 MB per detection, 1.6 GB in all (2026-10-13).
const maxEvents = 20

// Event is a detection, as kept to show it. Its frames are on disk, see EventFrame.
type Event struct {
	ID   int
	Time time.Time
	// Event is the detection without its frames (Before and After are nil).
	*vision.Event
	// Cameras are the names of the cameras by their index in the vision.Event.
	Cameras []string
	// Game is the game the server entered the dart in, 0 if it wasn't reported;
	// ReportErr is why the server didn't take it.
	Game      int64
	ReportErr error
}

// Events returns the latest detections, newest first.
func (a *App) Events() []Event {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	events := slices.Clone(a.events)
	slices.Reverse(events)
	return events
}

// Event returns the detection with the given ID.
func (a *App) Event(id int) (Event, bool) {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	for _, e := range a.events {
		if e.ID == id {
			return e, true
		}
	}
	return Event{}, false
}

// AddEvent saves a detection's frames and keeps the rest to show it. It returns its ID.
func (a *App) AddEvent(e Event) int {
	a.evMu.Lock()
	a.nextID++
	e.ID = a.nextID
	a.evMu.Unlock()
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	if err := a.saveFrames(e.ID, e.Cameras, e.Before, e.After); err != nil {
		log.Printf("save the frames of event %d: %v", e.ID, err)
	}
	withoutFrames := *e.Event
	withoutFrames.Before, withoutFrames.After = nil, nil
	e.Event = &withoutFrames

	a.evMu.Lock()
	// Frames are saved in the background, a later detection may be done first.
	i, _ := slices.BinarySearchFunc(a.events, e.ID, func(x Event, id int) int { return x.ID - id })
	a.events = slices.Insert(a.events, i, e)
	var dropped []Event
	if len(a.events) > maxEvents && !a.opts.KeepFrames {
		dropped = slices.Clone(a.events[:len(a.events)-maxEvents])
	}
	if len(a.events) > maxEvents {
		a.events = a.events[len(a.events)-maxEvents:]
	}
	a.evMu.Unlock()
	for _, d := range dropped {
		for i := range d.Cameras {
			for _, kind := range frameKinds {
				_ = os.Remove(a.framePath(d, i, kind))
			}
		}
	}
	return e.ID
}

// frameKinds are the frames saved per camera of a detection.
var frameKinds = []string{"before", "after"}

// framePath returns the file of a camera's frame of a detection, named like
// 003-cam1-before.jpg, as darts-o-cam analyze reads them.
func (a *App) framePath(e Event, camera int, kind string) string {
	return filepath.Join(a.opts.FrameDir, fmt.Sprintf("%03d-%s-%s.jpg", e.ID, e.Cameras[camera], kind))
}

func (a *App) saveFrames(id int, names []string, before, after []*image.RGBA) error {
	e := Event{ID: id, Cameras: names}
	for i := range names {
		for _, f := range []struct {
			kind   string
			frames []*image.RGBA
		}{{"before", before}, {"after", after}} {
			if i >= len(f.frames) || f.frames[i] == nil {
				continue
			}
			if err := WriteJPEG(a.framePath(e, i, f.kind), f.frames[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// EventFrame returns the file of a camera's frame before or after a detection
// (kind "before" or "after"); false if there is no such detection, camera or frame.
func (a *App) EventFrame(id, camera int, kind string) (string, bool) {
	e, ok := a.Event(id)
	if !ok || camera < 0 || camera >= len(e.Cameras) || !slices.Contains(frameKinds, kind) {
		return "", false
	}
	path := a.framePath(e, camera, kind)
	if _, err := os.Stat(path); err != nil {
		return "", false // not saved (yet)
	}
	return path, true
}

// JPEGQuality is the quality frames are saved with: high, so the detection on them works
// like on the original frames.
const JPEGQuality = 95

// WriteJPEG saves an image as JPEG.
func WriteJPEG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
