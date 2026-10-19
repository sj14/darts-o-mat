package camapp

import (
	"cmp"
	"errors"
	"image"
	"sync"

	"github.com/sj14/darts-o-mat/internal/cam/camera"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

// snapshot is a still frame of a device that isn't a board camera, to show which camera
// it is. Taking one opens the device for a moment.
type snapshot struct {
	mu    sync.Mutex // held while taking the frame
	frame *image.RGBA
	err   error
}

// Snapshot returns a frame of a connected device: the current one of a board camera, or
// for other devices one taken when first asked for, until ClearSnapshots.
func (a *App) Snapshot(id string) (*image.RGBA, error) {
	if live := a.liveOf(id); live != nil {
		img, err := live.Latest()
		if img == nil {
			return nil, cmp.Or(err, errNoFrame)
		}
		return img, nil
	}

	a.snapMu.Lock()
	s := a.snaps[id]
	if s == nil {
		s = &snapshot{}
		a.snaps[id] = s
	}
	s.mu.Lock() // the first caller takes the frame, the others wait for it
	a.snapMu.Unlock()
	defer s.mu.Unlock()
	if s.frame == nil && s.err == nil {
		a.mu.Lock()
		opts := camera.Options{Width: a.cfg.Width, Height: a.cfg.Height, FPS: a.cfg.FPS}
		a.mu.Unlock()
		var cam *camera.Camera
		if cam, s.frame, s.err = vision.OpenCamera(id, opts); s.err == nil {
			cam.Close()
		}
	}
	return s.frame, s.err
}

var errNoFrame = errors.New("no frame yet")

// liveOf returns the board camera using the device, nil if none does.
func (a *App) liveOf(id string) *vision.Live {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.cams {
		if c.live != nil && c.live.Device().ID == id {
			return c.live
		}
	}
	return nil
}

// ClearSnapshots drops the frames taken of devices, so they are taken again, e.g.
// after holding a hand in front of a camera to see which one it is.
func (a *App) ClearSnapshots() {
	a.snapMu.Lock()
	defer a.snapMu.Unlock()
	clear(a.snaps)
}

// forgetSnapshot drops the frame of a device, e.g. once it became a board camera.
func (a *App) forgetSnapshot(id string) {
	a.snapMu.Lock()
	defer a.snapMu.Unlock()
	delete(a.snaps, id)
}
