package camapp

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/warning"
)

// Status tells what a board's detection is doing.
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

// Status returns what the detection of a board is doing.
func (a *App) Status(board int) Status {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	return a.statuses[board]
}

func (a *App) setStatus(board int, s Status) {
	a.evMu.Lock()
	defer a.evMu.Unlock()
	a.statuses[board] = s
}

// run is a running detection.
type run struct {
	cancel context.CancelFunc
	done   chan struct{}
	// started is when it started; reconnected is set if it was started by reconnecting
	// the cameras after a failure (see reconnectAfter).
	started     time.Time
	reconnected bool
}

// reconnectWithin is how long a detection started by reconnecting the cameras has to
// run before it may reconnect again: a camera that fails again right away stays failed
// until the user reconnects, instead of the board reopening its cameras in a loop.
const reconnectWithin = time.Minute

// reconnectAfter reports whether the cameras are reconnected after the detection failed
// at the time: once, unless it was started by reconnecting and failed again soon.
func (r *run) reconnectAfter(failed time.Time) bool {
	return !r.reconnected || failed.Sub(r.started) >= reconnectWithin
}

// stopBoard ends the running detection of a board, if any. a.mu must be held.
func (a *App) stopBoard(id int) {
	r := a.runs[id]
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
	delete(a.runs, id)
}

// stop ends all running detections. a.mu must be held.
func (a *App) stop() {
	for id := range a.runs {
		a.stopBoard(id)
	}
}

// restart starts the detections over with the current cameras and settings. a.mu must
// be held.
func (a *App) restart() {
	a.stop()
	for i := range a.cfg.Boards {
		a.start(&a.cfg.Boards[i])
	}
}

// restartBoard starts the detection of one board over, leaving those of the other boards
// alone (they know which darts are in their board). a.mu must be held.
func (a *App) restartBoard(b *BoardConfig) {
	a.stopBoard(b.ID)
	a.start(b)
}

// start starts the detection of a board, if it is switched on. a.mu must be held, the
// board's detection stopped.
func (a *App) start(b *BoardConfig) {
	a.startRun(b, false)
}

// startRun is start; reconnected is set when it starts after the cameras were reconnected
// because the detection failed.
func (a *App) startRun(b *BoardConfig, reconnected bool) {
	var srv *server
	if b.Server != "" && b.Token != "" {
		srv = newServer(b.Server, b.Token)
	}
	// status also reports the board's warnings to the server. It is called from the
	// detection's goroutine too, so it doesn't read b, which a.mu guards.
	board, cameras := b.ID, len(b.Cameras)
	var small warning.List // of the cameras the detection uses, found below
	status := func(s Status) {
		a.setStatus(board, s)
		a.reportWarnings(board, srv, statusWarnings(cameras, s, small))
	}
	if !b.Detect {
		status(Status{})
		return
	}
	d := detection{board: b.ID, name: b.Name, opts: b.Detection, server: srv}
	if srv != nil {
		d.report = newReporter(b.Name, srv, a.setReport)
	}
	calib := a.calibration()
	for _, c := range b.Cameras {
		bc := a.cams[c.Name]
		// A camera whose view doesn't match its reference may be another one of the
		// same name, with the wrong calibration: not until it's calibrated again.
		if bc.live == nil || bc.problem != nil || !c.Calibrated(bc.width, bc.height) {
			continue
		}
		m, _ := c.Mapping()
		start, _ := c.ToBoard()
		d.lives = append(d.lives, bc.live)
		d.names = append(d.names, c.Name)
		d.maps = append(d.maps, m)
		d.starts = append(d.starts, start)
		if smallFrames(bc, b) {
			small = append(small, warning.SmallFrames{Camera: c.Name, Width: bc.width, Height: bc.height})
		}
		empty, err := calib.LoadEmpty(c)
		if err != nil {
			log.Printf("%s: image of the empty board: %v", c.Name, err)
		}
		d.empty = append(d.empty, empty)
	}
	if len(d.lives) == 0 {
		status(Status{Err: ErrNoCalibratedCamera, Reporting: d.report != nil})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &run{cancel: cancel, done: make(chan struct{}), started: time.Now(), reconnected: reconnected}
	a.runs[b.ID] = r
	running := Status{Running: true, Cameras: len(d.lives), Reporting: d.report != nil}
	d.warnings = statusWarnings(cameras, running, small)
	status(running)
	if d.report != nil {
		go d.report.run(ctx)
	}
	go func() {
		err := a.detect(ctx, d)
		stopped := ctx.Err() != nil // by the app: nothing went wrong
		if !stopped {
			log.Printf("%s: detection stopped: %v", d.name, err)
			status(Status{Err: err, Reporting: d.report != nil})
		}
		close(r.done) // before reconnecting: that takes a.mu, which stopBoard holds while it waits for done
		if !stopped && r.reconnectAfter(time.Now()) {
			a.reconnectFailed(d.board, r)
		}
	}()
}

// reconnectFailed opens the cameras of a board again after its detection r failed, e.g.
// a camera dropped off the USB for a moment, and starts the detection again, so a game
// goes on without the user noticing. Nothing happens if the detection was stopped or
// started again meanwhile.
func (a *App) reconnectFailed(board int, r *run) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(board)
	if b == nil || a.runs[board] != r {
		return
	}
	log.Printf("%s: reconnecting the cameras", b.Name)
	a.reopenBoard(b, true) // r is done: stopping it only forgets it
}

// detection is what a run of the detection works with; it doesn't change while it runs.
type detection struct {
	board  int // ID of the board
	name   string
	lives  []*vision.Live
	names  []string
	maps   []vision.Mapping
	opts   vision.WatchOptions
	empty  []*image.RGBA // nil entries for cameras without an image of the empty board
	report *reporter     // nil: don't report
	server *server       // nil: don't report
	// starts are the cameras' calibrations, to check the grid from (gridCheck).
	starts []vision.Homography
	// warnings are those of the running detection, before checking the grid.
	warnings warning.List
}

// detect watches the cameras and reports the events until ctx is done or a camera fails.
func (a *App) detect(ctx context.Context, d detection) error {
	w := vision.NewWatcher(d.maps, d.opts)
	if !hasAll(d.empty) {
		log.Printf("%s: not all cameras have an image of the empty board: start with an empty board, or pulling out darts that were already in it counts as a throw", d.name)
	} else {
		w.Empty = d.empty
	}
	var motionAt time.Time // when the board started moving, only with Verbose
	if a.opts.Verbose {
		w.OnMotion = func() {
			motionAt = time.Now()
			log.Printf("%s: motion", d.name)
		}
	}
	log.Printf("%s: watching %d cameras: %v", d.name, len(d.lives), d.names)

	seq := make([]uint64, len(d.lives))
	frames := make([]*image.RGBA, len(d.lives))
	grid := newGridCheck(d)
	for {
		// Analyze every frame once: wait for a new one of each camera.
		for i, l := range d.lives {
			var err error
			if frames[i], seq[i], err = l.Next(ctx, seq[i]); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("camera %s: %w", d.names[i], err)
			}
		}
		start := time.Now()
		if e := w.Feed(frames); e != nil {
			if a.opts.Verbose {
				log.Printf("%s: %s after the motion began, of which %s detecting", d.name,
					time.Since(motionAt).Round(time.Millisecond), time.Since(start).Round(time.Millisecond))
			}
			a.handle(d, e)
			// Without a server nobody would see the result.
			if e.Kind == vision.EventTakeout && d.server != nil {
				grid.start(e.After)
			}
		}
		if mismatch, ok := grid.done(); ok {
			a.reportWarnings(d.board, d.server, append(slices.Clone(d.warnings), mismatch...))
		}
	}
}

// handle logs, reports and keeps an event. The report is sent in the background (see
// reporter), the event shows what became of it.
func (a *App) handle(d detection, e *vision.Event) {
	ev := Event{ID: a.newEventID(), BoardID: d.board, Time: time.Now(), Event: e, Cameras: d.names, Sending: d.report != nil}
	switch e.Kind {
	case vision.EventDart:
		var cams strings.Builder
		for i, p := range e.Cameras {
			fmt.Fprintf(&cams, " %s=%.0f,%.0f", d.names[i], p.X, p.Y)
		}
		log.Printf("%s: dart: %s at %.1f,%.1f mm (%d cameras:%s)", d.name, e.Dart.String(), e.Board.X, e.Board.Y, len(e.Cameras), cams.String())
		if d.report != nil {
			// The position goes along for the statistics, if the cameras found it.
			dart := e.Dart
			if e.Located {
				dart.X, dart.Y = &e.Board.X, &e.Board.Y
			}
			d.report.add(report{event: ev.ID, at: ev.Time, dart: dart})
		}
	case vision.EventTakeout:
		log.Printf("%s: takeout", d.name)
		if d.report != nil {
			d.report.add(report{event: ev.ID, at: ev.Time, takeout: true})
		}
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
