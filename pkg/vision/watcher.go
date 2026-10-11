package vision

import (
	"image"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// WatchOptions tune when the board counts as moving or still.
type WatchOptions struct {
	Detect DetectOptions
	// SampleStep compares only every n-th pixel in both directions to find motion, which is much cheaper.
	SampleStep int
	// MotionThreshold is the difference (0-255) in any color channel for a sample to count as changed.
	// It is higher than the detection's threshold, because the samples are not blurred against noise.
	MotionThreshold uint8
	// MotionPixels is the number of changed samples in a camera that counts as motion.
	MotionPixels int
	// StableFrames is how many consecutive frames without motion are needed before evaluating.
	StableFrames int
	// TakeoutPixels is the number of changed samples (compared to before the change)
	// above which the change is a hand or darts being pulled out, not a new dart.
	TakeoutPixels int
}

var DefaultWatchOptions = WatchOptions{
	Detect:          DefaultDetectOptions,
	SampleStep:      4,
	MotionThreshold: 40,
	MotionPixels:    40,
	StableFrames:    3,
	TakeoutPixels:   4000,
}

type EventKind string

const (
	EventDart    EventKind = "dart"
	EventTakeout EventKind = "takeout"
)

// Event is something that happened on the board.
type Event struct {
	Kind  EventKind
	Board dartboard.Point // for darts: the fused position in mm
	Dart  throw.Dart
	// Cameras are the positions on the board in mm seen by the cameras that found the dart, by camera index.
	Cameras map[int]dartboard.Point
	// Before and After are the still frames of all cameras the event was decided on.
	Before, After []*image.RGBA
}

// Watcher turns the frames of all cameras of a board into events: after something
// changed and the board is still again, it decides whether a dart was added.
type Watcher struct {
	// OnMotion is called when the board starts moving, e.g. for logging.
	OnMotion func()

	toBoard []Homography
	opts    WatchOptions

	ref      []*image.RGBA // the still board before the current change
	prev     []*image.RGBA
	changing bool
	still    int
}

// NewWatcher watches cameras with the given image-to-board homographies.
func NewWatcher(toBoard []Homography, opts WatchOptions) *Watcher {
	return &Watcher{toBoard: toBoard, opts: opts}
}

// Feed takes the latest frame of every camera, in the same order as the homographies.
// It returns an event once a change is over, nil otherwise.
// The slice may be reused by the caller, the images must not be changed afterwards.
func (w *Watcher) Feed(frames []*image.RGBA) *Event {
	frames = slices.Clone(frames)
	defer func() { w.prev = frames }()
	if w.prev == nil {
		w.ref = frames
		return nil
	}

	moving := false
	for i, f := range frames {
		if w.changedSamples(w.prev[i], f) > w.opts.MotionPixels {
			moving = true
		}
	}
	if moving {
		if !w.changing && w.OnMotion != nil {
			w.OnMotion()
		}
		w.changing, w.still = true, 0
		return nil
	}
	if !w.changing {
		// Nothing happened: follow slow lighting changes.
		w.ref = frames
		return nil
	}
	if w.still++; w.still < w.opts.StableFrames {
		return nil
	}

	// The board is still again after a change.
	w.changing = false
	ref := w.ref
	w.ref = frames

	changed := 0
	for i, f := range frames {
		changed = max(changed, w.changedSamples(ref[i], f))
	}
	if changed <= w.opts.MotionPixels {
		return nil // e.g. a shadow passed by
	}
	if changed > w.opts.TakeoutPixels {
		return &Event{Kind: EventTakeout, Before: ref, After: frames}
	}

	e := &Event{Kind: EventDart, Cameras: map[int]dartboard.Point{}, Before: ref, After: frames}
	var points []dartboard.Point
	for i, f := range frames {
		d, err := DetectDart(ref[i], f, w.opts.Detect)
		if err != nil {
			continue
		}
		p := w.toBoard[i].Apply(d.Tip)
		e.Cameras[i] = p
		points = append(points, p)
	}
	board, err := FuseTips(points)
	if err != nil {
		// Something changed, but no camera saw a dart: most likely darts were pulled out.
		return &Event{Kind: EventTakeout, Before: ref, After: frames}
	}
	e.Board = board
	e.Dart = dartboard.Score(board)
	return e
}

// changedSamples counts the sampled pixels that changed clearly between two frames.
func (w *Watcher) changedSamples(a, b *image.RGBA) int {
	step, threshold := w.opts.SampleStep, int(w.opts.MotionThreshold)
	n := 0
	bounds := a.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			i := a.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				if abs(int(a.Pix[i+c])-int(b.Pix[i+c])) > threshold {
					n++
					break
				}
			}
		}
	}
	return n
}
