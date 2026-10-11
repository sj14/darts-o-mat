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

// MaxBoardRadius is the radius of the whole board including the number ring, in mm.
// A dart detected farther out is a misdetection.
const MaxBoardRadius = 225

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

	// darts marks, per camera, where the darts thrown since the last takeout are in the image.
	darts []*Mask

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
		return w.takeout(ref, frames)
	}

	// Comparing images can't tell a dart that appeared from one that disappeared.
	// If the change lies mostly where earlier darts are, they were pulled out.
	areas := make([][]image.Point, len(frames))
	var inside, total int
	for i, f := range frames {
		areas[i] = ChangedArea(ref[i], f, w.opts.Detect)
		total += len(areas[i])
		if i < len(w.darts) && w.darts[i] != nil {
			for _, p := range areas[i] {
				if w.darts[i].at(p.X, p.Y) {
					inside++
				}
			}
		}
	}
	if total > 0 && inside*2 > total {
		return w.takeout(ref, frames)
	}

	e := &Event{Kind: EventDart, Cameras: map[int]dartboard.Point{}, Before: ref, After: frames}
	var points []dartboard.Point
	for i, f := range frames {
		w.rememberDart(i, f.Bounds(), areas[i])
		d, err := DetectDart(ref[i], f, w.opts.Detect)
		if err != nil {
			continue
		}
		p := w.toBoard[i].Apply(d.Tip)
		if p.Radius() > MaxBoardRadius {
			continue // e.g. a reflection outside the board was taken for the dart
		}
		e.Cameras[i] = p
		points = append(points, p)
	}
	board, err := FuseTips(points)
	if err != nil {
		// Something changed, but no camera saw a dart: most likely darts were pulled out.
		return w.takeout(ref, frames)
	}
	e.Board = board
	e.Dart = dartboard.Score(board)
	return e
}

// takeout forgets the darts on the board.
func (w *Watcher) takeout(before, after []*image.RGBA) *Event {
	w.darts = nil
	return &Event{Kind: EventTakeout, Before: before, After: after}
}

// rememberDart marks where a new dart is in a camera's image, with some margin,
// because the dart may move a little when the next one hits.
func (w *Watcher) rememberDart(camera int, bounds image.Rectangle, area []image.Point) {
	for len(w.darts) <= camera {
		w.darts = append(w.darts, nil)
	}
	if w.darts[camera] == nil {
		w.darts[camera] = &Mask{W: bounds.Dx(), H: bounds.Dy(), Pix: make([]bool, bounds.Dx()*bounds.Dy())}
	}
	m := &Mask{W: bounds.Dx(), H: bounds.Dy(), Pix: make([]bool, bounds.Dx()*bounds.Dy())}
	for _, p := range area {
		m.Pix[p.Y*m.W+p.X] = true
	}
	grown := m.dilate(5)
	for i, set := range grown.Pix {
		if set {
			w.darts[camera].Pix[i] = true
		}
	}
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
