package camapp

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/sj14/darts-o-mat/pkg/vision"
)

// maxEvents is how many detections are kept to show them. Each keeps the frames of all
// cameras (about 22 MB for three cameras at 1280×720).
const maxEvents = 20

// Event is a detection, as kept to show it.
type Event struct {
	ID   int
	Time time.Time
	*vision.Event
	// Cameras are the names of the cameras by their index in the vision.Event.
	Cameras []string
	// Game is the game the server entered the dart in, 0 if it wasn't reported;
	// ReportErr is why the server didn't take it.
	Game      int64
	ReportErr error
}

// Status tells what the detection is doing.
type Status struct {
	// Running is whether the detection runs; Cameras is how many it watches.
	Running bool
	Cameras int
	// Err is why it doesn't run (ErrNoCalibratedCamera) or why it stopped.
	Err error
	// Reporting is whether darts are reported to a server.
	Reporting bool
}

// ErrNoCalibratedCamera means the detection waits for a calibrated camera.
var ErrNoCalibratedCamera = errors.New("no calibrated camera")

// Status returns what the detection is doing.
func (a *App) Status() Status {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	return a.status
}

func (a *App) setStatus(s Status) {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	a.status = s
}

// Events returns the latest detections, newest first.
func (a *App) Events() []Event {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	events := make([]Event, len(a.events))
	for i, e := range a.events {
		events[len(events)-1-i] = e
	}
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

// AddEvent keeps a detection to show it and returns its ID.
func (a *App) AddEvent(e Event) int {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	a.nextID++
	e.ID = a.nextID
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	a.events = append(a.events, e)
	if len(a.events) > maxEvents {
		a.events = a.events[len(a.events)-maxEvents:]
	}
	return e.ID
}

// run is a running detection.
type run struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// stop ends the running detection, if any. a.mu must be held.
func (a *App) stop() {
	if a.run == nil {
		return
	}
	a.run.cancel()
	<-a.run.done
	a.run = nil
}

// restart starts the detection over with the current cameras and settings, if it is
// switched on. a.mu must be held.
func (a *App) restart() {
	a.stop()
	if !a.cfg.Detect {
		a.setStatus(Status{})
		return
	}
	d := detection{toBoard: []vision.Homography{}, opts: a.cfg.Detection}
	if a.cfg.Server != "" && a.cfg.Token != "" {
		d.server = newServer(a.cfg.Server, a.cfg.Token)
	}
	calib := a.calibration()
	for i, c := range a.cfg.Cameras {
		bc := a.cams[i]
		// A camera whose view doesn't match its reference may be another one of the
		// same name, with the wrong calibration: not until it's calibrated again.
		if bc.live == nil || bc.problem != nil || !c.Calibrated(bc.width, bc.height) {
			continue
		}
		toBoard, _ := c.ToBoard()
		d.lives = append(d.lives, bc.live)
		d.names = append(d.names, c.Name)
		d.toBoard = append(d.toBoard, toBoard)
		empty, err := calib.LoadEmpty(c)
		if err != nil {
			log.Printf("%s: image of the empty board: %v", c.Name, err)
		}
		d.empty = append(d.empty, empty)
	}
	if len(d.lives) == 0 {
		a.setStatus(Status{Err: ErrNoCalibratedCamera, Reporting: d.server != nil})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &run{cancel: cancel, done: make(chan struct{})}
	a.run = r
	a.setStatus(Status{Running: true, Cameras: len(d.lives), Reporting: d.server != nil})
	go func() {
		defer close(r.done)
		err := a.detect(ctx, d)
		if ctx.Err() == nil {
			log.Printf("detection stopped: %v", err)
			a.setStatus(Status{Err: err, Reporting: d.server != nil})
		}
	}()
}

// detection is what a run of the detection works with; it doesn't change while it runs.
type detection struct {
	lives   []*vision.Live
	names   []string
	toBoard []vision.Homography
	opts    vision.WatchOptions
	empty   []*image.RGBA // nil entries for cameras without an image of the empty board
	server  *server       // nil: don't report
}

// detect watches the cameras and reports the events until ctx is done or a camera fails.
func (a *App) detect(ctx context.Context, d detection) error {
	w := vision.NewWatcher(d.toBoard, d.opts)
	if !hasAll(d.empty) {
		log.Printf("not all cameras have an image of the empty board: start with an empty board, or pulling out darts that were already in it counts as a throw")
	} else {
		w.Empty = d.empty
	}
	if a.opts.Verbose {
		w.OnMotion = func() { log.Printf("motion") }
	}
	log.Printf("watching %d cameras: %v", len(d.lives), d.names)

	seq := make([]uint64, len(d.lives))
	frames := make([]*image.RGBA, len(d.lives))
	for {
		// Analyze every frame once: wait for a new one of each camera.
		for i, l := range d.lives {
			var err error
			if frames[i], seq[i], err = l.Next(seq[i]); err != nil {
				return fmt.Errorf("camera %s: %w", d.names[i], err)
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e := w.Feed(frames); e != nil {
			a.handle(ctx, d, e)
		}
	}
}

// handle logs, reports and keeps an event.
func (a *App) handle(ctx context.Context, d detection, e *vision.Event) {
	ev := Event{Event: e, Cameras: d.names}
	switch e.Kind {
	case vision.EventDart:
		cams := ""
		for i, p := range e.Cameras {
			cams += fmt.Sprintf(" %s=%.0f,%.0f", d.names[i], p.X, p.Y)
		}
		log.Printf("dart: %s at %.1f,%.1f mm (%d cameras:%s)", e.Dart.String(), e.Board.X, e.Board.Y, len(e.Cameras), cams)
		if d.server != nil {
			// The position goes along for the statistics.
			dart := e.Dart
			dart.X, dart.Y = &e.Board.X, &e.Board.Y
			ev.Game, ev.ReportErr = d.server.dart(ctx, dart)
		}
	case vision.EventTakeout:
		log.Printf("takeout")
		if d.server != nil {
			ev.Game, ev.ReportErr = d.server.takeout(ctx)
		}
	}
	if ev.ReportErr != nil {
		log.Printf("  not reported: %v", ev.ReportErr)
	} else if ev.Game != 0 {
		log.Printf("  entered in game %d", ev.Game)
	}
	id := a.AddEvent(ev)
	if a.opts.SaveDir != "" {
		if err := saveEvent(a.opts.SaveDir, id, d.names, e); err != nil {
			log.Printf("save event: %v", err)
		}
	}
}

func hasAll(images []*image.RGBA) bool {
	for _, img := range images {
		if img == nil {
			return false
		}
	}
	return true
}

// saveEvent writes the frames before and after an event as JPEGs, e.g. 003-cam1-after.jpg.
func saveEvent(dir string, n int, names []string, e *vision.Event) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, name := range names {
		for kind, img := range map[string]*image.RGBA{"before": e.Before[i], "after": e.After[i]} {
			if err := WriteJPEG(filepath.Join(dir, fmt.Sprintf("%03d-%s-%s.jpg", n, name, kind)), img); err != nil {
				return err
			}
		}
	}
	return nil
}

// JPEGQuality is the quality frames are saved with for debugging: high, so the
// detection on them works like on the original frames.
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
