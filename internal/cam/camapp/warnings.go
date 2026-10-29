package camapp

import (
	"context"
	"image"
	"log"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/warning"
)

// warningReport is what darts-o-mat is told about a board: all its current warnings.
type warningReport struct {
	server   *server
	warnings warning.List
}

// statusWarnings are the warnings of a board with the given number of cameras whose
// detection has the status s; small are the cameras it uses whose frames are much smaller
// than wanted, which only count while it runs.
func statusWarnings(cameras int, s Status, small warning.List) warning.List {
	var ws warning.List
	if s.Cameras < cameras {
		ws = append(ws, warning.CamerasMissing{Cameras: cameras, Watching: s.Cameras})
	}
	if s.Running {
		ws = append(ws, small...)
	}
	return ws
}

// reportWarnings queues all current warnings of a board for sendWarnings; it doesn't wait.
// Only the newest warnings of a board are sent, and nothing is sent again: warnings the
// server didn't get stay lost until the next change. Without a server nothing is sent.
func (a *App) reportWarnings(board int, srv *server, ws warning.List) {
	if srv == nil {
		return
	}
	a.evMu.Lock()
	a.warningReports[board] = warningReport{server: srv, warnings: ws}
	a.evMu.Unlock()
	select {
	case a.warningsChanged <- struct{}{}:
	default: // the sender is told already
	}
}

// sendWarnings sends the queued warnings until ctx is done, one at a time, so newer
// warnings of a board never arrive before older ones.
func (a *App) sendWarnings(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-a.warningsChanged:
		}
		a.evMu.Lock()
		reports := a.warningReports
		a.warningReports = map[int]warningReport{}
		a.evMu.Unlock()
		for board, r := range reports {
			if err := r.server.warnings(ctx, r.warnings); err != nil && ctx.Err() == nil {
				log.Printf("board %d: warnings not reported: %v", board, err)
			}
		}
	}
}

// gridCheck checks after a takeout whether the calibration of each camera still fits the
// board, on the frames of the empty board: a camera that was moved gets a GridMismatch
// warning. It takes about 45 ms per camera at 1280×720 and runs in the background, so the
// detection doesn't wait; one check at a time. It belongs to the detection's goroutine,
// which collects the result with done: a check still running when the detection ends is
// dropped.
type gridCheck struct {
	board   string // name, for the log
	names   []string
	starts  []vision.Homography // the cameras' calibrations, see detection
	running bool
	result  chan warning.List
}

func newGridCheck(d detection) *gridCheck {
	return &gridCheck{board: d.name, names: d.names, starts: d.starts, result: make(chan warning.List, 1)}
}

// start checks the frames, one per camera, unless a check runs already.
func (c *gridCheck) start(frames []*image.RGBA) {
	if c.running {
		return
	}
	c.running = true
	go func() { c.result <- c.check(frames) }()
}

// done returns the cameras whose grid doesn't fit, once the check finished.
func (c *gridCheck) done() (warning.List, bool) {
	select {
	case grid := <-c.result:
		c.running = false
		return grid, true
	default:
		return nil, false
	}
}

func (c *gridCheck) check(frames []*image.RGBA) warning.List {
	var grid warning.List
	for i, img := range frames {
		res, err := vision.RefineCalibration(img, c.starts[i])
		switch {
		case err != nil:
			log.Printf("%s: the grid of %s doesn't fit the board anymore: %v", c.board, c.names[i], err)
		case res.FoundShare() < vision.MinFoundShare:
			log.Printf("%s: the grid of %s doesn't fit the board anymore: found %.0f %% of its features", c.board, c.names[i], res.FoundShare()*100)
		default:
			continue
		}
		grid = append(grid, warning.GridMismatch{Camera: c.names[i]})
	}
	return grid
}
