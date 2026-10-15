package vision

import (
	"image"
	"slices"
	"sync"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// WatchOptions tune when the board counts as moving or still. Like DetectOptions, sizes
// are on the board (mm, mm²) and converted to each camera's pixels.
type WatchOptions struct {
	Detect DetectOptions `json:"detect"`
	// SampleStep compares only pixels this far apart to find motion, which is much cheaper (mm).
	SampleStep float64 `json:"sample_step"`
	// MotionThreshold is the difference (0-255) in any color channel for a sample to count as changed.
	// It is higher than the detection's threshold, because the samples are not blurred against noise.
	MotionThreshold uint8 `json:"motion_threshold"`
	// MotionArea is the changed area in a camera that counts as motion (mm²).
	MotionArea float64 `json:"motion_area"`
	// StableFrames is how many consecutive frames without motion are needed before evaluating.
	StableFrames int `json:"stable_frames"`
	// TakeoutArea is the changed area (compared to before the change) above which the
	// change is a hand or darts being pulled out, not a new dart (mm²).
	TakeoutArea float64 `json:"takeout_area"`
}

// DefaultWatchOptions were measured in pixels on the development board (see ReferenceScale):
// every 4th pixel, motion from 40 changed samples, takeout from 4000.
var DefaultWatchOptions = WatchOptions{
	Detect:          DefaultDetectOptions,
	SampleStep:      px(4),
	MotionThreshold: 40,
	MotionArea:      px2(40 * 4 * 4),
	StableFrames:    3,
	TakeoutArea:     px2(4000 * 4 * 4),
}

// dartMargin is how far around a dart its pixels are remembered, because the dart may
// move a little when the next one hits (5 px on the development board).
var dartMargin = px(5)

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
	Board dartboard.Point // for darts: the fused position in mm; for a miss no camera located, zero
	// Located tells a dart whose position the cameras found from a miss they didn't
	// locate (see Watcher.Feed).
	Located bool
	Dart    throw.Dart
	// Cameras are the positions on the board seen by the cameras that found the dart, by camera index.
	Cameras map[int]Tip
	// Tips are where those cameras found the tip in their image, in pixels.
	Tips map[int]dartboard.Point
	// Before and After are the still frames of all cameras the event was decided on.
	Before, After []*image.RGBA
}

// Watcher turns the frames of all cameras of a board into events: after something
// changed and the board is still again, it decides whether a dart was added.
type Watcher struct {
	// OnMotion is called when the board starts moving, e.g. for logging.
	OnMotion func()
	// Empty are images of the empty board, one per camera (optional). With them, darts
	// pulled out are told apart from darts thrown even if they were in the board before
	// the watcher started; without them, only darts the watcher saw coming are known.
	Empty []*image.RGBA

	maps   []Mapping
	opts   WatchOptions
	params []watchParams // the options in each camera's pixels

	// darts marks, per camera, where the darts thrown since the last takeout are in the image.
	darts []*Mask

	ref      []*image.RGBA // the still board before the current change
	prev     []*image.RGBA
	changing bool
	still    int
}

// NewWatcher watches cameras with the given mappings of their images to the board.
func NewWatcher(maps []Mapping, opts WatchOptions) *Watcher {
	w := &Watcher{maps: maps, opts: opts}
	for _, m := range maps {
		w.params = append(w.params, opts.pixels(PixelScale(m.ToBoard)))
	}
	return w
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
		if w.changedSamples(i, w.prev[i], f) > w.params[i].motionSamples {
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

	changed, takeout := false, false
	for i, f := range frames {
		n := w.changedSamples(i, ref[i], f)
		changed = changed || n > w.params[i].motionSamples
		takeout = takeout || n > w.params[i].takeoutSamples
	}
	if !changed {
		return nil // e.g. a shadow passed by
	}
	if takeout {
		return w.takeout(ref, frames)
	}

	// Each camera is evaluated once, all at the same time: it takes a while per frame.
	areas := make([][]image.Point, len(frames))
	detections := make([]Detection, len(frames))
	errs := make([]error, len(frames))
	var wg sync.WaitGroup
	for i, f := range frames {
		var darts *Mask
		if i < len(w.darts) {
			darts = w.darts[i]
		}
		wg.Go(func() { detections[i], areas[i], errs[i] = detect(ref[i], f, w.params[i].detect, darts, nil) })
	}
	wg.Wait()

	// Comparing images can't tell a dart that appeared from one that disappeared.
	if w.removed(ref, frames, areas) {
		return w.takeout(ref, frames)
	}

	e := &Event{Kind: EventDart, Cameras: map[int]Tip{}, Tips: map[int]dartboard.Point{}, Before: ref, After: frames}
	var tips []Tip
	for i, f := range frames {
		w.rememberDart(i, f.Bounds(), areas[i])
		if errs[i] != nil {
			continue
		}
		d := detections[i]
		p := w.maps[i].Tip(d, w.opts.Detect.TipShift)
		if p.Radius() > MaxBoardRadius {
			continue // e.g. a reflection outside the board was taken for the dart
		}
		radial, tangential := w.maps[i].Resolution(p)
		tip := Tip{Point: p, Radial: radial, Tangential: tangential, Hidden: d.Hidden}
		e.Cameras[i] = tip
		e.Tips[i] = d.Tip
		tips = append(tips, tip)
	}
	board, err := FuseTips(tips)
	if err != nil {
		if len(w.Empty) != len(frames) {
			// Something changed, but no camera saw a dart: most likely darts were pulled out.
			return w.takeout(ref, frames)
		}
		// With the empty board, a takeout is told apart above: this was a dart no camera
		// located, e.g. far out in the surround, where a thin change along the board's rim
		// (the surround moved when hit) was taken for it (2026-10-13). A miss.
		e.Dart = throw.Dart{Segment: throw.Miss}
		return e
	}
	e.Board, e.Located = board, true
	e.Dart = dartboard.Score(board)
	return e
}

// removed reports whether the change took darts away rather than adding one.
// With images of the empty board: see PulledOut. Without them: the change lies mostly
// where darts seen earlier are.
func (w *Watcher) removed(before, after []*image.RGBA, areas [][]image.Point) bool {
	if len(w.Empty) == len(after) {
		return PulledOut(before, after, w.Empty, areas)
	}

	var inside, total int
	for i, area := range areas {
		total += len(area)
		if i < len(w.darts) && w.darts[i] != nil {
			for _, p := range area {
				if w.darts[i].at(p.X, p.Y) {
					inside++
				}
			}
		}
	}
	return total > 0 && inside*2 > total
}

// PulledOut reports whether a change took darts out of the board rather than adding
// one: the changed areas (per camera) look more like the empty board afterwards. Comparing
// images can't tell a dart that appeared from one that disappeared.
func PulledOut(before, after, empty []*image.RGBA, areas [][]image.Point) bool {
	// Each camera votes on its own: one camera whose exposure changed while a hand was
	// in front of it differs from the empty board almost everywhere and would outweigh
	// the others in a sum (seen on a real takeout: 0.06 and 0.08 vs. 1.9).
	var votes, cameras, beforeSum, afterSum int
	for i, area := range areas {
		if len(area) == 0 {
			continue
		}
		var beforeDiff, afterDiff int
		for _, p := range area {
			beforeDiff += pixelDiff(before[i], empty[i], p)
			afterDiff += pixelDiff(after[i], empty[i], p)
		}
		// Measured on real events: about 0.05 for darts pulled out (0.55 when a
		// neighboring dart moved, too), 4 to 17 for darts thrown.
		if afterDiff < beforeDiff {
			votes++
		}
		cameras++
		beforeSum += beforeDiff
		afterSum += afterDiff
	}
	if votes*2 == cameras {
		return afterSum < beforeSum // a tie
	}
	return votes*2 > cameras
}

// pixelDiff returns the largest difference of a pixel's color channels between two images.
func pixelDiff(a, b *image.RGBA, p image.Point) int {
	i, j := a.PixOffset(p.X, p.Y), b.PixOffset(p.X, p.Y)
	d := 0
	for c := 0; c < 3; c++ {
		d = max(d, abs(int(a.Pix[i+c])-int(b.Pix[j+c])))
	}
	return d
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
	grown := m.dilate(w.params[camera].dartMargin)
	for i, set := range grown.Pix {
		if set {
			w.darts[camera].Pix[i] = true
		}
	}
}

// changedSamples counts the sampled pixels that changed clearly between two frames of a camera.
func (w *Watcher) changedSamples(camera int, a, b *image.RGBA) int {
	step, threshold := w.params[camera].sampleStep, w.params[camera].motionThreshold
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
