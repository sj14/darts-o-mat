package camapp

import (
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"slices"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

// CameraTrace is a camera's detection of an event computed again, with its steps.
type CameraTrace struct {
	Name  string
	Trace vision.Trace
	After *image.RGBA // the frame after the dart
	Err   error       // why it couldn't be computed, e.g. the camera isn't calibrated anymore
}

// ErrNoTrace means the event is no dart detection (e.g. a takeout) or its frames are gone.
var ErrNoTrace = errors.New("no detection to trace")

// Traces computes the detection of a dart event again from its saved frames, recording
// each camera's steps (vision.TraceDart), to see why a detection went wrong. It is only
// computed on request: it takes about as long as the detection itself, per camera.
//
// The darts thrown before it since the last takeout are detected again from their
// frames too, so the detection knows where they are, like the watcher did; darts whose
// event was dropped already (see maxEvents) are missing. It uses the current calibration
// and settings, which may differ from those at the time of the throw.
func (a *App) Traces(id int) ([]CameraTrace, error) {
	a.evMu.Lock()
	events := slices.Clone(a.events) // oldest first
	a.evMu.Unlock()
	end := slices.IndexFunc(events, func(e Event) bool { return e.ID == id })
	if end < 0 || events[end].Kind != vision.EventDart {
		return nil, ErrNoTrace
	}
	start := end
	for start > 0 && events[start-1].Kind == vision.EventDart {
		start--
	}

	a.mu.Lock()
	calib, opts := a.calibration(), a.cfg.Detection.Detect
	a.mu.Unlock()

	earlier := map[string][]image.Point{}
	for _, e := range events[start:end] {
		for i, name := range e.Cameras {
			before, after, scale, err := a.traceInput(calib, e, i, name)
			if err != nil {
				continue
			}
			_, area, _ := vision.DetectDartBeside(before, after, opts, scale, earlier[name])
			earlier[name] = append(earlier[name], area...)
		}
	}

	e := events[end]
	traces := make([]CameraTrace, len(e.Cameras))
	for i, name := range e.Cameras {
		traces[i].Name = name
		before, after, scale, err := a.traceInput(calib, e, i, name)
		if err != nil {
			traces[i].Err = err
			continue
		}
		traces[i].After = after
		traces[i].Trace = vision.TraceDart(before, after, opts, scale, earlier[name])
	}
	return traces, nil
}

// traceInput returns a camera's saved frames of an event and its current scale.
func (a *App) traceInput(calib vision.Calibration, e Event, camera int, name string) (before, after *image.RGBA, scale float64, err error) {
	cam, err := calib.Camera(name)
	if err != nil {
		return nil, nil, 0, err
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("%s isn't calibrated: %w", name, err)
	}
	if before, err = ReadJPEG(a.framePath(e, camera, "before")); err != nil {
		return nil, nil, 0, err
	}
	if after, err = ReadJPEG(a.framePath(e, camera, "after")); err != nil {
		return nil, nil, 0, err
	}
	return before, after, vision.PixelScale(toBoard), nil
}

// ReadJPEG loads a saved frame.
func ReadJPEG(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		return nil, err
	}
	return vision.ToRGBA(img), nil
}
