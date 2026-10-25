package vision

import (
	"cmp"
	"errors"
	"image"
	"image/draw"
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// DetectOptions tune the dart detection. Sizes are on the board, in mm and mm², and
// converted to each camera's pixels with its scale (see PixelScale). The zero value is
// not useful, see DefaultDetectOptions.
type DetectOptions struct {
	// DiffThreshold is the difference (0-255) in any color channel above which a pixel counts as changed.
	DiffThreshold uint8 `json:"diff_threshold"`
	// WeakThreshold is a lower difference that only counts for pixels connected to
	// pixels above DiffThreshold. It keeps the faint, thin point attached to the
	// barrel without picking up noise elsewhere.
	WeakThreshold uint8 `json:"weak_threshold"`
	// Blur is the radius the images are smoothed with before comparing them, against noise (mm).
	Blur float64 `json:"blur"`
	// MinArea is the minimum size of the changed area to be a dart (mm²).
	MinArea float64 `json:"min_area"`
	// MinSpeck is the size below which separate changed specks are ignored as noise (mm²).
	MinSpeck float64 `json:"min_speck"`
	// MinElongation is the minimum ratio of the length to the width of a dart.
	MinElongation float64 `json:"min_elongation"`
	// Gap bridges gaps of up to twice this size when grouping changed pixels into
	// pieces: a barrel in front of black fields often only shows as a dashed line (mm).
	Gap float64 `json:"gap"`
	// AxisDistance is how far from the dart's axis a piece's center may be to belong to
	// the dart, like the flight above the barrel (mm).
	AxisDistance float64 `json:"axis_distance"`
	// TipShift is how far the tip found is moved back along the dart on the board (mm):
	// the change in the image tends to reach a little beyond the real tip, e.g. the
	// dart's shadow at the entry point (see Mapping.Tip).
	TipShift float64 `json:"tip_shift"`
}

// DefaultDetectOptions were measured in pixels on the development board (see ReferenceScale):
// blur radius 2 px, at least 150 px, specks below 15 px, gaps up to 6 px, axis distance 12 px.
// The tip shift is where the cameras agreed best on 13 real throws (2026-10-13).
var DefaultDetectOptions = DetectOptions{
	DiffThreshold: 25,
	WeakThreshold: 12,
	Blur:          px(2),
	MinArea:       px2(150),
	MinSpeck:      px2(15),
	MinElongation: 4,
	Gap:           px(3),
	AxisDistance:  px(12),
	TipShift:      1,
}

var (
	ErrNoDart       = errors.New("no dart found")
	ErrNotElongated = errors.New("changed area is not shaped like a dart")
)

// Detection is a dart found in one camera image.
type Detection struct {
	Tip    dartboard.Point // in image pixels
	Pixels int             // size of the changed area
	Axis   dartboard.Point // unit vector from the tip towards the flight, in image pixels
	// Hidden tells that the tip found ends on a dart thrown earlier: the dart's point
	// may be behind it, and the tip is just where it disappears (2026-10-13, event 37:
	// 7 mm too far inside, a T20 behind two darts in the 20). See hiddenDistance.
	Hidden bool
}

// ToRGBA converts an image to RGBA with its bounds starting at 0,0.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

// Mask marks the changed pixels of an image.
type Mask struct {
	W, H int
	Pix  []bool
}

func (m *Mask) at(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.W && y < m.H && m.Pix[y*m.W+x]
}

// near reports whether a pixel within r pixels of p is set.
func (m *Mask) near(p dartboard.Point, r int) bool {
	cx, cy := int(math.Round(p.X)), int(math.Round(p.Y))
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r && m.at(x, y) {
				return true
			}
		}
	}
	return false
}

// Count returns the number of changed pixels.
func (m *Mask) Count() int {
	n := 0
	for _, p := range m.Pix {
		if p {
			n++
		}
	}
	return n
}

// Diff returns the pixels where any color channel differs by more than threshold,
// plus the pixels above weak that are connected to them (hysteresis).
// Colors matter: a silver dart on the red or green bull has about the same brightness.
// Both images are blurred first, so that noise and tiny camera shakes don't count.
// Thin structures like the dart's point are kept, so there is no morphological cleanup.
func Diff(before, after *image.RGBA, threshold, weak uint8, blurRadius int) *Mask {
	b, a := blur(before, blurRadius), blur(after, blurRadius)
	w, h := b.Bounds().Dx(), b.Bounds().Dy()

	diff := make([]uint8, w*h)
	for y := range h {
		for x := range w {
			i := y*a.Stride + x*4
			var d int
			for c := range 3 {
				d = max(d, abs(int(a.Pix[i+c])-int(b.Pix[i+c])))
			}
			diff[y*w+x] = uint8(min(d, 255))
		}
	}

	m := &Mask{W: w, H: h, Pix: make([]bool, w*h)}
	var queue []int
	for i, d := range diff {
		if d > threshold {
			m.Pix[i] = true
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || yy < 0 || xx >= w || yy >= h {
					continue
				}
				j := yy*w + xx
				if !m.Pix[j] && diff[j] > weak {
					m.Pix[j] = true
					queue = append(queue, j)
				}
			}
		}
	}
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// DetectDart finds the dart that appeared between before and after and returns its tip.
// All changed areas except tiny specks count as the dart: it often falls apart into
// pieces where it crosses board areas of similar brightness. scale is the camera's
// pixels per board millimetre (see PixelScale).
func DetectDart(before, after *image.RGBA, opts DetectOptions, scale float64) (Detection, error) {
	d, _, err := DetectDartBeside(before, after, opts, scale, nil)
	return d, err
}

// DetectDartBeside is DetectDart with the darts thrown earlier: the changed pixels of
// their detections (as returned, since the last takeout). Like the Watcher, it tries
// without them if the change isn't shaped like a dart, see detect. It also returns the
// changed area of this dart.
func DetectDartBeside(before, after *image.RGBA, opts DetectOptions, scale float64, earlier []image.Point) (Detection, []image.Point, error) {
	return detect(before, after, opts.pixels(scale), dartsMask(before.Bounds(), earlier), nil)
}

// dartsMask marks the changed pixels of the darts thrown earlier; nil without any.
func dartsMask(b image.Rectangle, earlier []image.Point) *Mask {
	if len(earlier) == 0 {
		return nil
	}
	darts := &Mask{W: b.Dx(), H: b.Dy(), Pix: make([]bool, b.Dx()*b.Dy())}
	for _, p := range earlier {
		darts.Pix[p.Y*darts.W+p.X] = true
	}
	return darts
}

// hiddenDistance is how close a tip has to be to the changed pixels of a dart thrown
// earlier to count as hidden by it (Detection.Hidden). A point that disappears behind
// a dart ends where that dart's change begins, both blurred: 2.3 px at the reference
// scale (2026-10-22, event 211, cam3). The wider dartMargin is too much: a T18 free
// beside the dart before it (2026-10-25, event 41, cam3: 6.7 px away) counted as
// hidden, so the camera that saw the triple best was dropped.
var hiddenDistance = px(4)

// detect is DetectDart that also returns the changed area, so the watcher compares the
// images only once per camera.
//
// darts marks the changed pixels of the darts thrown earlier (nil: none). If the change
// isn't shaped like a dart, it tries again without the changes there, widened by
// dartMargin (the earlier dart may have moved a little): a dart hit by the new one moves, its outline changes too and can merge with the new dart into one piece
// that is too wide (2026-10-13, event 95: two parallel darts, elongation 3.1). Only then,
// because a new dart in front of an old one in the image would lose pixels.
//
// tr, if not nil, records the steps (see Trace).
func detect(before, after *image.RGBA, p detectParams, darts *Mask, tr *Trace) (Detection, []image.Point, error) {
	mask := Diff(before, after, p.diff, p.weak, p.blur)
	if tr != nil {
		tr.Changed, tr.Earlier = mask, darts
	}
	d, area, err := detectIn(mask, p, tr)
	if darts == nil {
		return d, area, err
	}
	if err == nil {
		d.Hidden = darts.near(d.Tip, p.hiddenDistance)
	}
	if err != ErrNotElongated {
		return d, area, err
	}
	widened := darts.dilate(p.dartMargin)
	without := &Mask{W: mask.W, H: mask.H, Pix: slices.Clone(mask.Pix)}
	for i, set := range widened.Pix {
		if set {
			without.Pix[i] = false
		}
	}
	retry := tr
	if tr != nil {
		retry = &Trace{Changed: tr.Changed, Earlier: widened, Retried: true}
	}
	if d2, area2, err2 := detectIn(without, p, retry); err2 == nil && len(area2) >= retryMinAreas*p.minPixels {
		if tr != nil {
			*tr = *retry
		}
		d2.Hidden = darts.near(d2.Tip, p.hiddenDistance)
		return d2, area2, nil
	}
	return d, area, err
}

// retryMinAreas is how many times MinArea the dart must be that is found when the earlier
// darts' pixels are taken out (see detect): what is left of a change can be a sliver along
// an earlier dart's edge (2026-10-18, event 199, cam3: 484 px of a change of 10,131 px,
// its "tip" 25 mm from where the other cameras saw it). The smallest of 45 real darts
// (events 181 to 199 and the test throws) had 1357 px, MinArea is 150 px, so five times
// it, 750 px, lies between them.
const retryMinAreas = 5

// detectIn finds the dart's tip in the changed pixels.
func detectIn(mask *Mask, p detectParams, tr *Trace) (Detection, []image.Point, error) {
	seed, area := dartArea(mask, p, tr)
	if len(area) < p.minPixels {
		return Detection{}, area, ErrNoDart
	}
	if seed == nil {
		return Detection{}, area, ErrNotElongated
	}
	d, err := tipOf(seed, area, tr)
	return d, area, err
}

// ChangedArea returns the changed pixels that DetectDart looks at, e.g. to debug a detection.
func ChangedArea(before, after *image.RGBA, opts DetectOptions, scale float64) []image.Point {
	p := opts.pixels(scale)
	_, area := dartArea(Diff(before, after, p.diff, p.weak, p.blur), p, nil)
	return area
}

// dartArea finds the pixels of the new dart. The changed area often falls apart: a dark
// shaft on a black field doesn't show, so flight and barrel become separate pieces. The
// seed is the largest elongated piece: barrel and point; a piece that includes the flight
// counts if its lower half is elongated. A barrel alone counts as a whole: its lower half
// is only half as long (2026-10-14, event 4: a dart passing behind an earlier dart's
// flight fell apart, its barrel piece had 3.9 in the lower half and 7.4 as a whole).
// Pieces on the seed's axis above its lower end, like the flight, belong to the dart;
// specks elsewhere don't. Below the seed is
// board: a speck there on the axis (color noise at a ring's edge, 56 px below a real
// dart's tip, 2026-10-13) would become the tip. Small gaps in the point are bridged
// already (GapRadius); no real throw had a piece of the dart below the seed.
func dartArea(mask *Mask, p detectParams, tr *Trace) (seed, area []image.Point) {
	var comps [][]image.Point
	for _, c := range mask.componentsBridging(p.gap) {
		if len(c) >= p.minSpeck {
			comps = append(comps, c)
		}
	}
	slices.SortFunc(comps, func(a, b []image.Point) int { return len(b) - len(a) })
	if tr != nil {
		tr.Pieces = comps
		defer func() { tr.Seed, tr.Area = seed, area }()
	}

	seedIndex := -1
	for i, c := range comps {
		if lower := lowerHalf(c); len(lower) >= 10 {
			_, _, e := principalAxis(lower)
			_, _, whole := principalAxis(c)
			if tr != nil && i == 0 {
				tr.Elongation = max(e, whole) // of the largest piece, also when it isn't a dart
			}
			if max(e, whole) >= p.minElongation {
				if tr != nil {
					tr.Elongation = max(e, whole)
				}
				seedIndex = i
				break
			}
		}
	}
	if seedIndex < 0 {
		// Nothing dart-like; return everything for debugging.
		for _, c := range comps {
			area = append(area, c...)
		}
		return nil, area
	}

	seed = comps[seedIndex]
	center, axis, _ := principalAxis(lowerHalf(seed))
	axis = downwards(axis)
	end := extent(seed, axis) // the seed's lower end along the axis
	area = slices.Clone(seed) // its own center may be off the axis, e.g. with a big flight
	for i, c := range comps {
		if i == seedIndex {
			continue
		}
		cc, _, _ := principalAxis(c)
		// Distance of the piece's center from the axis line, and where the piece starts.
		onAxis := math.Abs((cc.X-center.X)*axis.Y-(cc.Y-center.Y)*axis.X) <= float64(p.axisDistance)
		if onAxis && -extent(c, dartboard.Point{X: -axis.X, Y: -axis.Y}) <= end {
			area = append(area, c...)
		}
	}
	return seed, area
}

// downwards turns an axis to point down in the image, towards the tip.
func downwards(axis dartboard.Point) dartboard.Point {
	if axis.Y < 0 || (axis.Y == 0 && axis.X < 0) {
		return dartboard.Point{X: -axis.X, Y: -axis.Y}
	}
	return axis
}

// extent returns how far the points reach in a direction: their largest projection on it.
func extent(pts []image.Point, dir dartboard.Point) float64 {
	m := math.Inf(-1)
	for _, p := range pts {
		m = math.Max(m, float64(p.X)*dir.X+float64(p.Y)*dir.Y)
	}
	return m
}

// lowerHalf returns the pixels in the lower half of the area's height: for a dart that's
// the barrel and the point, without the wide flight, which would skew the axis.
func lowerHalf(area []image.Point) []image.Point {
	minY, maxY := area[0].Y, area[0].Y
	for _, p := range area {
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	mid := (minY + maxY) / 2
	var lower []image.Point
	for _, p := range area {
		if p.Y >= mid {
			lower = append(lower, p)
		}
	}
	return lower
}

// principalAxis returns the center of the points, the unit vector of their main
// direction and how elongated they are (length to width, +Inf for a line).
func principalAxis(pts []image.Point) (center, axis dartboard.Point, elongation float64) {
	n := float64(len(pts))
	for _, p := range pts {
		center.X += float64(p.X)
		center.Y += float64(p.Y)
	}
	center.X /= n
	center.Y /= n

	var sxx, syy, sxy float64
	for _, p := range pts {
		dx, dy := float64(p.X)-center.X, float64(p.Y)-center.Y
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
	}
	sxx /= n
	syy /= n
	sxy /= n

	// Eigenvalues and the eigenvector of the larger one.
	tr, det := sxx+syy, sxx*syy-sxy*sxy
	disc := math.Sqrt(math.Max(tr*tr/4-det, 0))
	l1, l2 := tr/2+disc, tr/2-disc
	axis = dartboard.Point{X: sxy, Y: l1 - sxx}
	if math.Abs(sxy) < 1e-9 {
		axis = dartboard.Point{X: 1}
		if syy > sxx {
			axis = dartboard.Point{Y: 1}
		}
	}
	norm := math.Hypot(axis.X, axis.Y)
	axis.X /= norm
	axis.Y /= norm

	elongation = math.Inf(1)
	if l2 > 0 {
		elongation = math.Sqrt(l1 / l2)
	}
	return center, axis, elongation
}

// tipAxis returns a point on the dart's axis and its direction towards the tip. The lower
// half of the seed in the image is barrel and point for a dart standing up in the image,
// but includes part of the flight for one lying flat (event 66 of 2026-10-14: the tip
// found was 15 px from the point, in the double); so the axis is fitted again, twice, to
// the part of the seed nearest the tip (tipAxisPart of its length along the axis).
func tipAxis(seed []image.Point) (center, axis dartboard.Point) {
	center, axis, _ = principalAxis(lowerHalf(seed))
	axis = downwards(axis)
	for range 2 {
		end := extent(seed, axis)
		length := end + extent(seed, dartboard.Point{X: -axis.X, Y: -axis.Y})
		var near []image.Point
		for _, p := range seed {
			if float64(p.X)*axis.X+float64(p.Y)*axis.Y >= end-tipAxisPart*length {
				near = append(near, p)
			}
		}
		if len(near) < 10 {
			break
		}
		c, a, _ := principalAxis(near)
		if a.X*axis.X+a.Y*axis.Y < 0 {
			a = dartboard.Point{X: -a.X, Y: -a.Y} // keep pointing towards the tip
		}
		center, axis = c, a
	}
	return center, axis
}

// tipAxisPart is the part of the seed's length the axis is fitted to: barrel and point
// without the flight. 0.3–0.5 gave the same fields on the measurement series and on 20
// throws (2026-10-14).
const tipAxisPart = 0.4

// tipOf finds the tip of the dart: the lower end of the area along the seed's axis.
// The cameras look across the board from its rim, so a dart sticking out of the
// board points up in the image.
func tipOf(seed, area []image.Point, tr *Trace) (Detection, error) {
	center, axis := tipAxis(seed)
	if tr != nil {
		c, a, _ := principalAxis(lowerHalf(seed))
		tr.FirstAxis = Line{c, downwards(a)}
		tr.Axis = Line{center, axis}
	}

	ts := make([]float64, len(area))
	for i, p := range area {
		ts[i] = float64(p.X)*axis.X + float64(p.Y)*axis.Y
	}
	sorted := slices.Clone(ts)
	slices.Sort(sorted)
	// Skip the 3 most extreme pixels, so single stray pixels don't move the tip.
	// (A percentile would cut off too much: the point is only about 2 px wide.)
	tipT := sorted[max(len(sorted)-4, 0)]

	// The tip is where the pixels within 3 px of the tip end are on average along the
	// axis (moving it to the very end was tried on real throws in 2026-10 and didn't
	// help), on the axis line through the barrel: across the axis these pixels are
	// skewed by the point's shadow or a wire beside it (event 33 of 2026-10-14: 4 px
	// beside the point, on the wire between 16 and 7 instead of in the 7).
	var t float64
	var count int
	for i, p := range area {
		if math.Abs(ts[i]-tipT) <= 3 {
			t += ts[i]
			count++
			if tr != nil {
				tr.TipEnd = append(tr.TipEnd, p)
			}
		}
	}
	if count == 0 {
		return Detection{}, ErrNoDart
	}
	t = t/float64(count) - (center.X*axis.X + center.Y*axis.Y)
	return Detection{
		Tip:    dartboard.Point{X: center.X + t*axis.X, Y: center.Y + t*axis.Y},
		Pixels: len(area),
		Axis:   dartboard.Point{X: -axis.X, Y: -axis.Y},
	}, nil
}

// Tip is a dart's position on the board as one camera sees it.
type Tip struct {
	dartboard.Point
	// Radial and Tangential are the camera's resolution there: image pixels per mm
	// towards the center and along the ring (see Mapping.Resolution). A camera counts
	// with them: the same spot can be 0.6 px/mm in one camera and 2.3 in another, and a
	// position found to about a pixel is that much more precise. Not with their square
	// (until 2026-10-15): then one camera outweighed two that agreed, e.g. a miss just
	// outside the D6 (cam1 169.8 mm at 2.2 px/mm, cam2 176.8 and cam3 179.5 at 1.3 and
	// 1.0) scored D6. 0 counts as 1.
	Radial, Tangential float64
	// Hidden tells that the camera may not see the tip, see Detection.Hidden.
	Hidden bool
}

// FuseTips combines the dart positions on the board seen by several cameras.
// The board is made of rings and segments, so it combines the distances to the center
// and the angles, each weighted by the cameras' resolution in that direction:
//
//   - With three cameras or more, the weighted median: a single camera that is far off
//     (e.g. because the dart hides its tip) doesn't move the result, and when cameras
//     with most of the weight see the dart beyond a ring's wire, so does the result.
//     (The median of each coordinate combined x of one camera with y of another, which
//     put a dart seen in the triple by two cameras closer to the center than both: 99.6
//     and 100.2 mm became 98.0. A weighted mean put that real T18, cameras 99.6, 100.2
//     and 96.3 mm, at 98.7 mm.)
//   - With two cameras, the weighted mean: there is no majority, and the median of two
//     would jump to the heavier one entirely. A real S14 seen by a camera with 0.7 px/mm
//     at 166 mm and one with 2.1 px/mm at 157 mm: 159.4 mm instead of 161.6, just next
//     to the double.
//
// Near the center the cameras surround the dart and their angles say nothing: then it
// combines each coordinate.
//
// Cameras whose tip may be hidden only count if no other camera found the dart, or if
// only two did and they are too far apart to tell which one is right (see
// disagree): the mean of two such cameras is a spot neither of them saw.
//
// A camera far from that result is dropped and the others are combined again.
// Otherwise the median of three takes whichever of the two good cameras is closer to
// the bad one, instead of the middle between them.
func FuseTips(tips []Tip) (dartboard.Point, error) {
	if visible := slices.DeleteFunc(slices.Clone(tips), func(t Tip) bool { return t.Hidden }); len(visible) > 0 && !disagree(visible) {
		tips = visible
	}
	p, err := fuse(tips)
	if err != nil || len(tips) < 3 {
		return p, err
	}
	var inliers []Tip
	for _, t := range tips {
		if math.Hypot(t.X-p.X, t.Y-p.Y) <= maxCameraDeviation {
			inliers = append(inliers, t)
		}
	}
	if len(inliers) < 2 || len(inliers) == len(tips) {
		return p, nil
	}
	return fuse(inliers)
}

// maxCameraDeviation is how far a camera may be from the fused position. Cameras that
// found the tip were measured up to 16 mm apart; one that only saw part of a dart
// hidden behind another was 30 mm off.
const maxCameraDeviation = 20.0

// disagree tells whether two cameras are too far apart for their mean: so far that at
// least one of them is more than maxCameraDeviation from it. Real case (2026-10-22, event
// 211): the third dart into a group in the 14, cam1 at 24.9 mm in the 17 and cam2 at
// 69.5 mm in the 14, on opposite sides of the bull; their mean, 46.9 mm, was a 16. cam3,
// whose tip ended on an earlier dart, saw the 14 at 33.1 mm.
func disagree(tips []Tip) bool {
	return len(tips) == 2 && math.Hypot(tips[0].X-tips[1].X, tips[0].Y-tips[1].Y) > 2*maxCameraDeviation
}

func fuse(tips []Tip) (dartboard.Point, error) {
	if len(tips) == 0 {
		return dartboard.Point{}, ErrNoDart
	}
	n := len(tips)
	xs, ys, radii, angles := make([]float64, n), make([]float64, n), make([]float64, n), make([]float64, n)
	wr, wt, wxy := make([]float64, n), make([]float64, n), make([]float64, n)
	var sumX, sumY float64
	for i, t := range tips {
		xs[i], ys[i], radii[i] = t.X, t.Y, t.Radius()
		sumX, sumY = sumX+t.X, sumY+t.Y
		wr[i], wt[i] = weight(t.Radial), weight(t.Tangential)
		wxy[i] = (wr[i] + wt[i]) / 2
	}
	r := combine(radii, wr)
	if r < polarFuseRadius {
		return dartboard.Point{X: combine(xs, wxy), Y: combine(ys, wxy)}, nil
	}
	// Angles relative to the mean direction, so they don't wrap around at ±180°.
	mean := math.Atan2(sumY, sumX)
	for i, t := range tips {
		angles[i] = math.Remainder(math.Atan2(t.Y, t.X)-mean, 2*math.Pi)
	}
	a := mean + combine(angles, wt)
	return dartboard.Point{X: r * math.Cos(a), Y: r * math.Sin(a)}, nil
}

// weight is a camera's weight for its resolution (px/mm) in one direction.
func weight(resolution float64) float64 {
	if resolution <= 0 {
		return 1
	}
	return resolution
}

// combine returns the weighted median of the values, for two the weighted mean.
func combine(values, weights []float64) float64 {
	if len(values) == 2 {
		return (values[0]*weights[0] + values[1]*weights[1]) / (weights[0] + weights[1])
	}
	order := make([]int, len(values))
	var total float64
	for i := range order {
		order[i] = i
		total += weights[i]
	}
	slices.SortFunc(order, func(a, b int) int { return cmp.Compare(values[a], values[b]) })
	var sum float64
	for k, i := range order {
		sum += weights[i]
		switch {
		case sum > total/2+1e-9:
			return values[i]
		case sum > total/2-1e-9 && k+1 < len(order):
			// Exactly half: in the middle, like the median of an even number.
			return (values[i] + values[order[k+1]]) / 2
		}
	}
	return values[order[len(order)-1]]
}

// polarFuseRadius is the distance from the center from which the cameras' angles
// agree well enough to fuse in polar coordinates (cameras are a few mm apart).
const polarFuseRadius = 2 * dartboard.BullRadius

// blur averages each pixel with its (2r+1)x(2r+1) neighborhood, per color channel.
// The frames of three cameras must be blurred quickly after every dart, so it is
// separable, uses running sums (the cost doesn't depend on r), goes through memory
// row by row, and divides by multiplying with the reciprocal.
func blur(src *image.RGBA, r int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	// div[n] turns a sum of n values into their average: (sum*div[n])>>32 is sum/n,
	// exactly, for sums below 2^32/n.
	div := make([]uint64, 2*r+2)
	for n := 1; n < len(div); n++ {
		div[n] = (1<<32 + uint64(n) - 1) / uint64(n)
	}

	// Rows: running sum along each row.
	tmp := make([]uint32, w*h*3)
	for y := range h {
		row := src.Pix[y*src.Stride : y*src.Stride+w*4]
		out := tmp[y*w*3 : (y+1)*w*3]
		var sr, sg, sb uint32
		n := 0
		for x := 0; x <= r && x < w; x++ {
			sr, sg, sb = sr+uint32(row[x*4]), sg+uint32(row[x*4+1]), sb+uint32(row[x*4+2])
			n++
		}
		for x := range w {
			d := div[n]
			out[x*3] = uint32(uint64(sr) * d >> 32)
			out[x*3+1] = uint32(uint64(sg) * d >> 32)
			out[x*3+2] = uint32(uint64(sb) * d >> 32)
			if add := x + r + 1; add < w {
				sr, sg, sb = sr+uint32(row[add*4]), sg+uint32(row[add*4+1]), sb+uint32(row[add*4+2])
				n++
			}
			if remove := x - r; remove >= 0 {
				sr, sg, sb = sr-uint32(row[remove*4]), sg-uint32(row[remove*4+1]), sb-uint32(row[remove*4+2])
				n--
			}
		}
	}

	// Columns: running sums of whole rows.
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	sums := make([]uint32, w*3)
	n := 0
	for y := 0; y <= r && y < h; y++ {
		for i, v := range tmp[y*w*3 : (y+1)*w*3] {
			sums[i] += v
		}
		n++
	}
	for y := range h {
		dst := out.Pix[y*out.Stride:]
		for x := range w {
			dst[x*4] = uint8(uint64(sums[x*3]) * div[n] >> 32)
			dst[x*4+1] = uint8(uint64(sums[x*3+1]) * div[n] >> 32)
			dst[x*4+2] = uint8(uint64(sums[x*3+2]) * div[n] >> 32)
			dst[x*4+3] = 255
		}
		if add := y + r + 1; add < h {
			for i, v := range tmp[add*w*3 : (add+1)*w*3] {
				sums[i] += v
			}
			n++
		}
		if remove := y - r; remove >= 0 {
			for i, v := range tmp[remove*w*3 : (remove+1)*w*3] {
				sums[i] -= v
			}
			n--
		}
	}
	return out
}

// componentsBridging returns the connected areas of the mask like components, but
// pixels up to 2r apart count as connected. Only the mask's own pixels are returned.
func (m *Mask) componentsBridging(r int) [][]image.Point {
	if r <= 0 {
		return m.components()
	}
	grown := m.dilate(r)
	var result [][]image.Point
	for _, c := range grown.components() {
		var own []image.Point
		for _, p := range c {
			if m.Pix[p.Y*m.W+p.X] {
				own = append(own, p)
			}
		}
		if len(own) > 0 {
			result = append(result, own)
		}
	}
	return result
}

// dilate grows every set pixel to a (2r+1)x(2r+1) square, separably for speed.
func (m *Mask) dilate(r int) *Mask {
	tmp := make([]bool, len(m.Pix))
	for y := 0; y < m.H; y++ {
		run := -1 // last set pixel in this row
		for x := 0; x < m.W+r; x++ {
			if x < m.W && m.Pix[y*m.W+x] {
				run = x
			}
			if xx := x - r; xx >= 0 && xx < m.W && run >= 0 && x-run <= 2*r {
				tmp[y*m.W+xx] = true
			}
		}
	}
	out := &Mask{W: m.W, H: m.H, Pix: make([]bool, len(m.Pix))}
	for x := 0; x < m.W; x++ {
		run := -1
		for y := 0; y < m.H+r; y++ {
			if y < m.H && tmp[y*m.W+x] {
				run = y
			}
			if yy := y - r; yy >= 0 && yy < m.H && run >= 0 && y-run <= 2*r {
				out.Pix[yy*m.W+x] = true
			}
		}
	}
	return out
}

// components returns the connected areas of the mask.
func (m *Mask) components() [][]image.Point {
	seen := make([]bool, len(m.Pix))
	var result [][]image.Point
	var queue []image.Point
	for i, set := range m.Pix {
		if !set || seen[i] {
			continue
		}
		var comp []image.Point
		queue = append(queue[:0], image.Point{X: i % m.W, Y: i / m.W})
		seen[i] = true
		for len(queue) > 0 {
			p := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			comp = append(comp, p)
			for _, d := range [8]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
				q := p.Add(d)
				j := q.Y*m.W + q.X
				if m.at(q.X, q.Y) && !seen[j] {
					seen[j] = true
					queue = append(queue, q)
				}
			}
		}
		result = append(result, comp)
	}
	return result
}
