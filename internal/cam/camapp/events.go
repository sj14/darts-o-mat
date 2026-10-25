package camapp

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

// maxEvents is how many detections of a board are shown. Their frames are kept in memory
// as JPEG (about 1.2 MB per detection with three cameras at 1280×720), not on disk: a
// Raspberry Pi's SD card would be written ~1 GB on a long evening. Not as images either:
// those took about 22 MB per detection, 1.6 GB in all (2026-10-13).
const maxEvents = 20

// Event is a detection, as kept to show it. Its frames are JPEG, see EventFrame.
type Event struct {
	ID int
	// BoardID is the board it was detected at.
	BoardID int
	Time    time.Time
	// Event is the detection without its frames (Before and After are nil).
	*vision.Event
	// Cameras are the names of the cameras by their index in the vision.Event.
	Cameras []string
	// Game is the game the server entered the dart in, 0 if it wasn't reported;
	// ReportErr is why the server didn't take it. While Sending, the server hasn't
	// answered yet; ReportErr is then why the last try failed (see reporter).
	Game      int64
	ReportErr error
	Sending   bool

	frames map[frameKey][]byte // JPEG
}

// frameKey names a camera's frame of a detection: kind is one of frameKinds.
type frameKey struct {
	camera int
	kind   string
}

// reportResult is what became of reporting an event, see setReport.
type reportResult struct {
	game    int64
	err     error
	sending bool
}

// newEventID returns the ID for a new detection, before AddEvent keeps it: its report is
// sent at once and may be answered before.
func (a *App) newEventID() int {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	a.nextID++
	return a.nextID
}

// setReport records what became of reporting a detection (reporter.result) and tells the
// pages showing it.
func (a *App) setReport(r report, game int64, err error, sending bool) {
	res := reportResult{game: game, err: err, sending: sending}
	a.evMu.Lock()
	if i := slices.IndexFunc(a.events, func(e Event) bool { return e.ID == r.event }); i >= 0 {
		a.events[i].apply(res)
	} else {
		a.reports[r.event] = res // AddEvent is still saving its frames
	}
	a.evMu.Unlock()
	a.newEvent.Notify()
}

func (e *Event) apply(r reportResult) {
	e.Game, e.ReportErr, e.Sending = r.game, r.err, r.sending
}

// Events returns the latest detections at a board, newest first.
func (a *App) Events(board int) []Event {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	events := a.boardEvents(board)
	slices.Reverse(events)
	return events
}

// boardEvents returns the detections at a board, oldest first. a.evMu must be held.
func (a *App) boardEvents(board int) []Event {
	return slices.DeleteFunc(slices.Clone(a.events), func(e Event) bool { return e.BoardID != board })
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

// AddEvent keeps a detection to show it, its frames as JPEG (and with Options.SaveDir
// also on disk). It returns its ID, a new one unless e has one from newEventID.
func (a *App) AddEvent(e Event) int {
	if e.ID == 0 {
		e.ID = a.newEventID()
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	e.frames = encodeFrames(e.Before, e.After)
	if a.opts.SaveDir != "" {
		if err := a.saveFrames(e); err != nil {
			log.Printf("save the frames of event %d: %v", e.ID, err)
		}
	}
	withoutFrames := *e.Event
	withoutFrames.Before, withoutFrames.After = nil, nil
	e.Event = &withoutFrames

	a.evMu.Lock()
	if r, ok := a.reports[e.ID]; ok {
		e.apply(r)
		delete(a.reports, e.ID)
	}
	// Frames are encoded in the background, a later detection may be done first.
	i, _ := slices.BinarySearchFunc(a.events, e.ID, func(x Event, id int) int { return x.ID - id })
	a.events = slices.Insert(a.events, i, e)
	if own := a.boardEvents(e.BoardID); len(own) > maxEvents {
		old := own[:len(own)-maxEvents]
		a.events = slices.DeleteFunc(a.events, func(x Event) bool {
			return slices.ContainsFunc(old, func(o Event) bool { return o.ID == x.ID })
		})
	}
	a.evMu.Unlock()
	a.newEvent.Notify()
	return e.ID
}

// WatchEvents returns a channel that receives a value after new detections, e.g. to
// update a page showing them, and a function to stop watching.
func (a *App) WatchEvents() (<-chan struct{}, func()) {
	return a.newEvent.Subscribe()
}

// frameKinds are the frames kept per camera of a detection.
var frameKinds = []string{"before", "after"}

// encodeFrames returns the frames before and after a detection as JPEG.
func encodeFrames(before, after []*image.RGBA) map[frameKey][]byte {
	frames := map[frameKey][]byte{}
	for kind, imgs := range map[string][]*image.RGBA{"before": before, "after": after} {
		for i, img := range imgs {
			if img == nil {
				continue
			}
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality}); err != nil {
				continue // only a frame larger than JPEG allows
			}
			frames[frameKey{i, kind}] = buf.Bytes()
		}
	}
	return frames
}

// FrameName returns the file name of a camera's frame of a detection, like
// 003-cam1-before.jpg, as darts-o-cam analyze reads them.
func (e Event) FrameName(camera int, kind string) string {
	return fmt.Sprintf("%03d-%s-%s.jpg", e.ID, e.Cameras[camera], kind)
}

// saveFrames writes a detection's frames into Options.SaveDir.
func (a *App) saveFrames(e Event) error {
	for k, data := range e.frames {
		if err := os.WriteFile(filepath.Join(a.opts.SaveDir, e.FrameName(k.camera, k.kind)), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// EventFrame returns a camera's frame before or after a detection (kind "before" or
// "after") as JPEG, and the detection; false if there is no such detection, camera or
// frame.
func (a *App) EventFrame(id, camera int, kind string) ([]byte, Event, bool) {
	e, ok := a.Event(id)
	if !ok || camera < 0 || camera >= len(e.Cameras) {
		return nil, Event{}, false
	}
	data, ok := e.frames[frameKey{camera, kind}]
	return data, e, ok
}

// JPEGQuality is the quality frames are kept with: high, so the detection on them works
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

// DecodeJPEG decodes a kept frame.
func DecodeJPEG(data []byte) (*image.RGBA, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return vision.ToRGBA(img), nil
}

// frame decodes a camera's frame of the detection (kind "before" or "after").
func (e Event) frame(camera int, kind string) (*image.RGBA, error) {
	data, ok := e.frames[frameKey{camera, kind}]
	if !ok {
		return nil, fmt.Errorf("no frame %s of %s", kind, e.Cameras[camera])
	}
	return DecodeJPEG(data)
}
