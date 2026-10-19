package camapp

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"strings"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

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
	d := detection{opts: a.cfg.Detection}
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
		m, _ := c.Mapping()
		d.lives = append(d.lives, bc.live)
		d.names = append(d.names, c.Name)
		d.maps = append(d.maps, m)
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
	lives  []*vision.Live
	names  []string
	maps   []vision.Mapping
	opts   vision.WatchOptions
	empty  []*image.RGBA // nil entries for cameras without an image of the empty board
	server *server       // nil: don't report
}

// detect watches the cameras and reports the events until ctx is done or a camera fails.
func (a *App) detect(ctx context.Context, d detection) error {
	w := vision.NewWatcher(d.maps, d.opts)
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
		var cams strings.Builder
		for i, p := range e.Cameras {
			fmt.Fprintf(&cams, " %s=%.0f,%.0f", d.names[i], p.X, p.Y)
		}
		log.Printf("dart: %s at %.1f,%.1f mm (%d cameras:%s)", e.Dart.String(), e.Board.X, e.Board.Y, len(e.Cameras), cams.String())
		if d.server != nil {
			// The position goes along for the statistics, if the cameras found it.
			dart := e.Dart
			if e.Located {
				dart.X, dart.Y = &e.Board.X, &e.Board.Y
			}
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
	// Saving the frames takes a moment: the detection goes on meanwhile.
	go a.AddEvent(ev)
}

func hasAll(images []*image.RGBA) bool {
	for _, img := range images {
		if img == nil {
			return false
		}
	}
	return true
}
