package vision

import (
	"errors"
	"image"
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// RefineResult describes how well a refined calibration fits the image.
type RefineResult struct {
	ToBoard Homography
	// Correction moves the homography's grid onto the rings found, see GridCorrection.
	Correction *GridCorrection
	// Matches is the number of board features found in the image and used for the fit.
	Matches int
	// Searched is the number of features searched for. If far fewer were found (Features),
	// the calibration doesn't fit the image, e.g. a wire was mixed up when clicking.
	Searched int
	// RMSBefore and RMSAfter are the root mean square distances in pixels between the
	// features found in the image and where the calibration expects them.
	RMSBefore, RMSAfter float64
	// Features are the features found in the last round, to show them for checking.
	Features []Feature
	// EdgeShift is how far the edges of the red and the green rings are found shifted
	// from the wires into the neighboring fields, in mm (see RefineCalibration).
	EdgeShift struct{ Red, Green float64 }
}

// Mapping returns the refined calibration with its correction.
func (r RefineResult) Mapping() Mapping {
	return Mapping{ToBoard: r.ToBoard, Correction: r.Correction}
}

// Feature is a ring edge or wire found in the image.
type Feature struct {
	Found dartboard.Point // in image pixels
	// Expected is where the refined calibration (with its correction) puts the feature in the image.
	Expected dartboard.Point
	// Deviation is the distance between both across the edge, in pixels.
	Deviation float64
	// Wire tells a wire between two numbers from a ring edge.
	Wire bool
	// Outlier is set for features left out of the fit, e.g. hidden by a dart.
	Outlier bool
}

var ErrTooFewMatches = errors.New("too few board features found in the image")

// RefineCalibration improves a rough calibration (e.g. from a few points clicked by hand)
// with hundreds of features of the board found in an image of it, ideally without darts:
//
//   - the edges of the colored rings (bullseye, bull, triple and double), found as jumps
//     in color saturation across the ring: the rings are red/green, the fields black/white;
//   - the wires between the numbers, found as the change from red to green (or back)
//     along the triple and double ring.
//
// Starting from the rough calibration, it searches each feature near its expected position,
// fits a new homography to all features found, drops outliers (darts, reflections) and
// repeats a few times with a narrower search.
//
// The edges between a colored ring and a field aren't found exactly at the wire: on the
// development board the red rings (next to black fields) came out 0.5–1 mm wider on each
// side, the green ones (next to white fields) slightly narrower, the same in all cameras
// (2026-10-13). Fitting the rings as they are then pulled the grid ±1 mm off around the
// board. So each round also estimates this shift per color, as how far the edges lie into
// the fields, and fits the edges where they are found: a homography can't widen or
// narrow the rings, so the shift is told apart from it. The grid is then at the wires.
func RefineCalibration(img image.Image, rough Homography) (RefineResult, error) {
	f := newFeatureImage(ToRGBA(img))
	toImage, err := rough.Inverse()
	if err != nil {
		return RefineResult{}, err
	}

	result := RefineResult{}
	var shift edgeShift
	windows := []float64{8, 6, 4, 3, 3, 3, 3, 3}
	for iteration, window := range windows {
		feats, searched := f.findFeatures(toImage, window)
		if len(feats) < 40 {
			return RefineResult{}, ErrTooFewMatches
		}
		if iteration == 0 {
			result.RMSBefore = normalRMS(toImage, feats)
		}
		shift.apply(feats)

		// Fit, then drop the worst matches (e.g. features hidden by a dart) and fit again.
		h, err := fitNormal(toImage, feats)
		if err != nil {
			return RefineResult{}, err
		}
		kept := dropOutliers(h, feats)
		if h, err = fitNormal(h, kept); err != nil {
			return RefineResult{}, err
		}
		toImage = h
		result.Matches, result.Searched = len(kept), searched
		result.RMSAfter = normalRMS(h, kept)
		if shift, err = estimateEdgeShift(h, kept); err != nil {
			return RefineResult{}, err
		}

		if iteration == len(windows)-1 {
			if result.Correction, err = correction(h, kept); err != nil {
				return RefineResult{}, err
			}
			limit := 0.0
			for _, ft := range kept {
				limit = max(limit, math.Abs(ft.residual(h)))
			}
			toBoard, _ := h.Inverse()
			m := Mapping{ToBoard: toBoard, Correction: result.Correction}
			result.Features = nil
			for _, ft := range feats {
				expected := h.Apply(ft.board)
				if !ft.wire {
					expected, _ = m.ToImage(ft.board)
				}
				result.Features = append(result.Features, Feature{
					Found:     ft.found,
					Expected:  expected,
					Deviation: math.Abs((expected.X-ft.found.X)*ft.normal.X + (expected.Y-ft.found.Y)*ft.normal.Y),
					Wire:      ft.wire,
					Outlier:   math.Abs(ft.residual(h)) > limit,
				})
			}
		}
	}

	result.EdgeShift.Red, result.EdgeShift.Green = shift[red], shift[green]
	result.ToBoard, err = toImage.Inverse()
	return result, err
}

// correction measures the grid correction on the ring edges used for the fit.
func correction(toImage Homography, feats []feature) (*GridCorrection, error) {
	toBoard, err := toImage.Inverse()
	if err != nil {
		return nil, err
	}
	var edges []ringEdge
	for _, ft := range feats {
		if !ft.wire {
			edges = append(edges, ringEdge{board: ft.board, found: ft.found, ring: ft.ring})
		}
	}
	return newGridCorrection(toBoard, edges), nil
}

// Ring colors, to index an edgeShift.
const (
	noColor = iota
	red
	green
)

// edgeShift is, per ring color, how far the edges between a colored ring and a field are
// found from the wire into the field, in mm.
type edgeShift [3]float64

// apply moves the features' board positions to where the edges are found.
func (s edgeShift) apply(feats []feature) {
	for i, ft := range feats {
		if ft.color != noColor {
			feats[i].board = dartboard.Polar(ft.ring+ft.toField*s[ft.color], ft.board.Angle())
		}
	}
}

// estimateEdgeShift measures the shift per color as the median distance from the wire
// into the field of the edges found, with toImage fitted to the shifted edges. It
// converges over the rounds of RefineCalibration.
func estimateEdgeShift(toImage Homography, feats []feature) (edgeShift, error) {
	toBoard, err := toImage.Inverse()
	if err != nil {
		return edgeShift{}, err
	}
	var into [3][]float64
	for _, ft := range feats {
		if ft.color != noColor {
			into[ft.color] = append(into[ft.color], (toBoard.Apply(ft.found).Radius()-ft.ring)*ft.toField)
		}
	}
	var s edgeShift
	for c, d := range into {
		if len(d) > 0 {
			slices.Sort(d)
			s[c] = d[len(d)/2]
		}
	}
	return s, nil
}

// feature is a point of the board found in the image. An edge only tells where the
// point is across the edge, so only the distance along normal counts when fitting.
type feature struct {
	board  dartboard.Point // in mm
	found  dartboard.Point // in pixels
	normal dartboard.Point // unit vector in the image, across the edge
	wire   bool            // a wire between numbers, not a ring edge

	// ring is the radius of a ring edge's wire. For edges between a colored ring and a
	// field: the ring's color and the direction from the ring into the field (+1
	// outwards, -1 inwards).
	ring    float64
	color   int
	toField float64
}

// residual is the signed distance of the feature from where toImage expects it, across the edge.
func (ft feature) residual(toImage Homography) float64 {
	p := toImage.Apply(ft.board)
	return (p.X-ft.found.X)*ft.normal.X + (p.Y-ft.found.Y)*ft.normal.Y
}

// fitNormal adjusts the homography (board to image) so that the features' distances
// across their edges become as small as possible, with a few Gauss-Newton steps.
func fitNormal(start Homography, feats []feature) (Homography, error) {
	// Board coordinates are scaled to about ±1 to keep the equations well conditioned.
	const scale = 1.0 / dartboard.DoubleOuterRadius
	h := start.Mul(Homography{1 / scale, 0, 0, 0, 1 / scale, 0, 0, 0, 1}).scaled()

	for range 5 {
		var jtj [8][8]float64
		var jtr [8]float64
		for _, ft := range feats {
			x, y := ft.board.X*scale, ft.board.Y*scale
			w := h[6]*x + h[7]*y + h[8]
			u := (h[0]*x + h[1]*y + h[2]) / w
			v := (h[3]*x + h[4]*y + h[5]) / w
			r := (u-ft.found.X)*ft.normal.X + (v-ft.found.Y)*ft.normal.Y
			nx, ny := ft.normal.X/w, ft.normal.Y/w
			j := [8]float64{
				nx * x, nx * y, nx,
				ny * x, ny * y, ny,
				-(nx*u + ny*v) * x, -(nx*u + ny*v) * y,
			}
			for a := range 8 {
				jtr[a] += j[a] * r
				for b := range 8 {
					jtj[a][b] += j[a] * j[b]
				}
			}
		}
		for a := range jtr {
			jtr[a] = -jtr[a]
		}
		delta, err := solve(jtj, jtr)
		if err != nil {
			return Homography{}, err
		}
		for a := range 8 {
			h[a] += delta[a]
		}
	}
	return h.Mul(Homography{scale, 0, 0, 0, scale, 0, 0, 0, 1}).scaled(), nil
}

// featureImage holds per-pixel signals the board features are found in.
type featureImage struct {
	w, h int
	sat  []float64 // color saturation: high on the red/green rings, low on black/white fields
	rg   []float64 // red minus green: positive on red rings, negative on green ones
}

func newFeatureImage(img *image.RGBA) *featureImage {
	b := blur(img, 1)
	w, h := b.Bounds().Dx(), b.Bounds().Dy()
	f := &featureImage{w: w, h: h, sat: make([]float64, w*h), rg: make([]float64, w*h)}
	for y := range h {
		for x := range w {
			i := b.PixOffset(x, y)
			r, g, bl := float64(b.Pix[i]), float64(b.Pix[i+1]), float64(b.Pix[i+2])
			f.sat[y*w+x] = max(r, g, bl) - min(r, g, bl)
			f.rg[y*w+x] = r - g
		}
	}
	return f
}

// at samples a signal with bilinear interpolation, ok is false outside the image.
func (f *featureImage) at(signal []float64, p dartboard.Point) (float64, bool) {
	x0, y0 := math.Floor(p.X), math.Floor(p.Y)
	if x0 < 0 || y0 < 0 || int(x0)+1 >= f.w || int(y0)+1 >= f.h {
		return 0, false
	}
	fx, fy := p.X-x0, p.Y-y0
	i := int(y0)*f.w + int(x0)
	v := signal[i]*(1-fx)*(1-fy) + signal[i+1]*fx*(1-fy) + signal[i+f.w]*(1-fx)*fy + signal[i+f.w+1]*fx*fy
	return v, true
}

// edge searches along dir around p for the strongest change of the signal with the
// expected sign (+1 rising, -1 falling) and returns its position with subpixel accuracy.
func (f *featureImage) edge(signal []float64, p, dir dartboard.Point, window float64, sign float64, minStrength float64) (dartboard.Point, bool) {
	const step = 0.5
	n := int(window / step)
	values := make([]float64, 0, 2*n+1)
	for i := -n; i <= n; i++ {
		v, ok := f.at(signal, dartboard.Point{X: p.X + dir.X*step*float64(i), Y: p.Y + dir.Y*step*float64(i)})
		if !ok {
			return dartboard.Point{}, false
		}
		values = append(values, v)
	}

	best, bestI := 0.0, -1
	for i := 1; i < len(values)-1; i++ {
		d := sign * (values[i+1] - values[i-1]) / (2 * step)
		if d > best {
			best, bestI = d, i
		}
	}
	if bestI < 0 || best < minStrength {
		return dartboard.Point{}, false
	}

	// Subpixel: fit a parabola through the derivative around the peak.
	offset := 0.0
	if bestI > 1 && bestI < len(values)-2 {
		dm := sign * (values[bestI] - values[bestI-2]) / (2 * step)
		dp := sign * (values[bestI+2] - values[bestI]) / (2 * step)
		if denom := dm - 2*best + dp; denom < 0 {
			offset = 0.5 * (dm - dp) / denom
		}
	}
	t := (float64(bestI-n) + offset) * step
	return dartboard.Point{X: p.X + dir.X*t, Y: p.Y + dir.Y*t}, true
}

// ringEdges are the circles whose edges show as a jump in saturation, with the
// direction of the jump going outwards: +1 from black/white into a colored ring.
var ringEdges = []struct {
	radius float64
	sign   float64
}{
	{dartboard.BullRadius, -1},
	{dartboard.TripleInnerRadius, +1},
	{dartboard.TripleOuterRadius, -1},
	{dartboard.DoubleInnerRadius, +1},
	{dartboard.DoubleOuterRadius, -1},
}

// allRingRadii are all circles of the board, to keep searches from jumping between them.
var allRingRadii = []float64{
	dartboard.BullseyeRadius, dartboard.BullRadius,
	dartboard.TripleInnerRadius, dartboard.TripleOuterRadius,
	dartboard.DoubleInnerRadius, dartboard.DoubleOuterRadius,
}

func dist(a, b dartboard.Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

// findFeatures searches the board's features near where toImage expects them and
// returns the ones found and how many it searched.
func (f *featureImage) findFeatures(toImage Homography, window float64) (feats []feature, searched int) {
	add := func(b, i, normal dartboard.Point) {
		feats = append(feats, feature{board: b, found: i, normal: normal})
	}
	// unit returns the image direction of moving from b1 to b2 on the board.
	unit := func(b1, b2 dartboard.Point) dartboard.Point {
		p1, p2 := toImage.Apply(b1), toImage.Apply(b2)
		d := dartboard.Point{X: p2.X - p1.X, Y: p2.Y - p1.Y}
		n := math.Hypot(d.X, d.Y)
		return dartboard.Point{X: d.X / n, Y: d.Y / n}
	}

	// radialWindow limits the search across a ring edge to less than half the distance
	// to the next edge, so the search can't jump to it (the bull is small in the image).
	radialWindow := func(r, a float64) float64 {
		gap := math.Inf(1)
		for _, other := range allRingRadii {
			if other != r {
				gap = min(gap, math.Abs(other-r))
			}
		}
		pxPerMM := dist(toImage.Apply(dartboard.Polar(r-1, a)), toImage.Apply(dartboard.Polar(r+1, a))) / 2
		return min(window, 0.45*gap*pxPerMM)
	}

	// Ring edges, every 3°, but away from the wires which interrupt the rings. All of
	// them border a field: the bull is green, the 20 has red rings, then the colors
	// alternate clockwise.
	for _, ring := range ringEdges {
		for a := 0.0; a < 360; a += 3 {
			if off := math.Mod(a+dartboard.SegmentAngle/2, dartboard.SegmentAngle); off < 2.5 || off > dartboard.SegmentAngle-2.5 {
				continue
			}
			b := dartboard.Polar(ring.radius, a)
			dir := unit(dartboard.Polar(ring.radius-1, a), dartboard.Polar(ring.radius+1, a))
			searched++
			if p, ok := f.edge(f.sat, toImage.Apply(b), dir, radialWindow(ring.radius, a), ring.sign, 8); ok {
				add(b, p, dir)
				ft := &feats[len(feats)-1]
				ft.color, ft.ring, ft.toField = green, ring.radius, -ring.sign
				if ring.radius != dartboard.BullRadius && int((a+dartboard.SegmentAngle/2)/dartboard.SegmentAngle)%2 == 0 {
					ft.color = red
				}
			}
		}
	}

	// Bullseye edge: red inside, green outside.
	for a := 0.0; a < 360; a += 10 {
		b := dartboard.Polar(dartboard.BullseyeRadius, a)
		dir := unit(dartboard.Polar(dartboard.BullseyeRadius-1, a), dartboard.Polar(dartboard.BullseyeRadius+1, a))
		searched++
		if p, ok := f.edge(f.rg, toImage.Apply(b), dir, radialWindow(dartboard.BullseyeRadius, a), -1, 8); ok {
			add(b, p, dir)
			feats[len(feats)-1].ring = dartboard.BullseyeRadius
		}
	}

	// Wires: along the colored rings, the color changes from red to green or back.
	// The 20 has red rings, then the colors alternate clockwise.
	for i := range dartboard.Segments {
		a := (float64(i) + 0.5) * dartboard.SegmentAngle
		sign := -1.0 // red to green
		if i%2 == 1 {
			sign = +1
		}
		for _, r := range []float64{101, 103, 105, 164, 166, 168} {
			b := dartboard.Polar(r, a)
			dir := unit(dartboard.Polar(r, a-0.5), dartboard.Polar(r, a+0.5))
			searched++
			if p, ok := f.edge(f.rg, toImage.Apply(b), dir, window, sign, 8); ok {
				add(b, p, dir)
				feats[len(feats)-1].wire = true
			}
		}
	}
	return feats, searched
}

// dropOutliers removes the features farther from the fit than 2.5 times the median
// distance, at least 0.5 px, e.g. features hidden by a dart.
func dropOutliers(toImage Homography, feats []feature) []feature {
	dists := make([]float64, len(feats))
	for i, ft := range feats {
		dists[i] = math.Abs(ft.residual(toImage))
	}
	sorted := slices.Clone(dists)
	slices.Sort(sorted)
	limit := max(2.5*sorted[len(sorted)/2], 0.5)

	var kept []feature
	for i, ft := range feats {
		if dists[i] <= limit {
			kept = append(kept, ft)
		}
	}
	return kept
}

func normalRMS(toImage Homography, feats []feature) float64 {
	var sum float64
	for _, ft := range feats {
		r := ft.residual(toImage)
		sum += r * r
	}
	return math.Sqrt(sum / float64(len(feats)))
}
