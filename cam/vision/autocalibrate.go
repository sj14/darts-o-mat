package vision

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/sj14/darts-o-mat/common/dartboard"
)

// ErrNoBoard means AutoCalibrate didn't find the rings and the bull of the board where it
// was told to look.
var ErrNoBoard = errors.New("the rings and the bull of the board weren't found")

// AutoResult is a calibration found by AutoCalibrate.
type AutoResult struct {
	RefineResult
	// Points are rough calibration points read off the result, so that it can be kept and
	// refined again like a calibration clicked by hand.
	Points []CalibrationPoint
}

// FoundShare is the share of the searched board features that were found: close to 1 if
// the grid fits the image, far less if it doesn't (see RefineResult.Searched).
func (r RefineResult) FoundShare() float64 {
	if r.Searched == 0 {
		return 0
	}
	return float64(r.Matches) / float64(r.Searched)
}

// autoRotations are how far the rotation is varied from what the tap says, in degrees: the
// tap is somewhere in the double of the 20, which is ±9° wide, and the fine calibration
// copes with a rotation that is off by up to about 6° (measured on the development board:
// ±6° always worked, ±9° only sometimes). With these three starts the one that is nearest
// is at most 3° off. A wrong rotation finds 10–25 % of the features, the right one 85 %
// and more, so which one fits is clear.
var autoRotations = []float64{0, -6, 6}

// autoFound is the share of features found from which a start is good enough not to try
// the others.
const autoFound = 0.85

// autoPointTargets are the wire crossings the rough points are read off for.
var autoPointTargets = [][2]uint8{{20, 1}, {6, 10}, {3, 19}, {11, 14}}

// AutoCalibrate finds a camera's calibration from an image of the empty board and one tap,
// anywhere in the double ring of the 20 (the outer red band at the top of the board's own
// numbering). Instead of four wire crossings clicked by hand it finds
//
//   - the outer edge of the double ring as an ellipse: the red and green pixels connected to
//     the tap, fitted by their outer edge;
//   - the bull's eye in the image. Because of the perspective it is not at the ellipse's
//     center; the board's center and the ellipse together say how the plane is seen, as
//     the center's polar line with respect to the ellipse is the plane's vanishing line;
//
// which fixes the homography up to a rotation, that the tap gives (and a reflection, which
// is always the same: the image's y axis points down, the board's up). It is only rough,
// 2–3 mm off on the development board's three cameras, and RefineCalibration does the rest.
// If the result doesn't fit the image (FoundShare), the caller falls back to points clicked
// by hand.
func AutoCalibrate(img image.Image, tap dartboard.Point) (AutoResult, error) {
	rgba := ToRGBA(img)
	w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()
	mask := redGreenMask(rgba)
	diagonal := math.Hypot(float64(w), float64(h))

	ring := ringPixels(mask, w, h, tap, diagonal)
	if len(ring) < 500 {
		return AutoResult{}, ErrNoBoard
	}
	e, ok := outerEdgeEllipse(ring)
	if !ok || !e.plausible(diagonal) {
		return AutoResult{}, ErrNoBoard
	}
	bull, ok := findBull(mask, w, h, e)
	if !ok {
		return AutoResult{}, ErrNoBoard
	}

	// Which way the 20 lies on the board and where the tap is seen under rotation 0.
	home, ok := polarHomography(e, bull, 0)
	if !ok {
		return AutoResult{}, ErrNoBoard
	}
	seen := home.Apply(tap)
	twenty := dartboard.Polar(1, 0)
	// The rotation comes before the reflection, which turns it around.
	theta := math.Atan2(seen.Y, seen.X) - math.Atan2(twenty.Y, twenty.X)

	var best AutoResult
	var lastErr error
	for _, offset := range autoRotations {
		start, ok := polarHomography(e, bull, theta+offset*math.Pi/180)
		if !ok {
			continue
		}
		res, err := RefineCalibration(rgba, start)
		if err != nil {
			lastErr = err
			continue
		}
		if best.Searched == 0 || res.FoundShare() > best.FoundShare() {
			best = AutoResult{RefineResult: res}
		}
		if res.FoundShare() >= autoFound {
			break
		}
	}
	if best.Searched == 0 {
		if lastErr != nil {
			return AutoResult{}, fmt.Errorf("%w: %w", ErrNoBoard, lastErr)
		}
		return AutoResult{}, ErrNoBoard
	}
	best.Points = pointsOf(best.Mapping())
	return best, nil
}

// pointsOf reads the rough calibration points off a calibration: where the wires cross the
// outer edge of the double ring in the image.
func pointsOf(m Mapping) []CalibrationPoint {
	var points []CalibrationPoint
	for _, between := range autoPointTargets {
		p := CalibrationPoint{Between: between, Ring: RingDoubleOuter}
		board, err := p.Board()
		if err != nil {
			continue
		}
		if p.Image, err = m.ToImage(board); err != nil {
			continue
		}
		points = append(points, p)
	}
	return points
}

// labA is the red-green channel a* of the CIE Lab color space: positive for red, negative
// for green and near zero for black, white and gray, whatever the brightness. That is what
// makes the board's rings stand out in a frame that is washed out on one side: the green
// of the far side of the development board has a saturation of 0.24 and a* of -17, the
// wall behind it, which is red-brown, +8 at most.
func labA(r, g, b uint8) (a, l float64) {
	R, G, B := srgbLinear[r], srgbLinear[g], srgbLinear[b]
	x := (0.4124*R + 0.3576*G + 0.1805*B) / 0.95047
	y := 0.2126*R + 0.7152*G + 0.0722*B
	fx, fy := labF(x), labF(y)
	return 500 * (fx - fy), 116*fy - 16
}

func labF(t float64) float64 {
	if t > 0.008856 {
		return math.Cbrt(t)
	}
	return 7.787*t + 16.0/116
}

var srgbLinear = func() (t [256]float64) {
	for i := range t {
		v := float64(i) / 255
		if v <= 0.04045 {
			t[i] = v / 12.92
		} else {
			t[i] = math.Pow((v+0.055)/1.055, 2.4)
		}
	}
	return
}()

// Thresholds of redGreenMask: how far from neutral a pixel is on the red-green axis, and
// the least brightness (L of Lab) it needs, so that a black field isn't taken for a ring.
const (
	ringChroma     = 14
	ringBrightness = 12
)

// fringeWidth is the radius of the opening of the mask: colored lines up to 2·fringeWidth
// pixels wide are removed. A camera with chromatic aberration colors the edges of the white
// numbers and the silver wires red and green, one or two pixels wide; the ring pixels would
// then reach over the wires into the triple ring and the numbers, and the ellipse would be
// fitted around the board's rim (measured on a Winmau Blade 6 with a flat view and such a
// camera: 15–40 % of the features found instead of 90 %). The double ring is wider.
const fringeWidth = 1

// redGreenMask marks the pixels that are red or green: the rings and the bull.
func redGreenMask(img *image.RGBA) []bool {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	mask := make([]bool, w*h)
	for y := range h {
		row := img.Pix[y*img.Stride:]
		for x := range w {
			a, l := labA(row[x*4], row[x*4+1], row[x*4+2])
			mask[y*w+x] = l > ringBrightness && math.Abs(a) > ringChroma
		}
	}
	return open(mask, w, h, fringeWidth)
}

// open removes from the mask what is thinner than 2r+1 pixels: erosion by a square of that
// size, then dilation.
func open(mask []bool, w, h, r int) []bool {
	eroded := make([]bool, w*h)
	for y := range h {
		for x := range w {
			eroded[y*w+x] = allSet(mask, w, h, x, y, r)
		}
	}
	opened := make([]bool, w*h)
	for y := range h {
		for x := range w {
			if !eroded[y*w+x] {
				continue
			}
			for yy := max(0, y-r); yy <= min(h-1, y+r); yy++ {
				for xx := max(0, x-r); xx <= min(w-1, x+r); xx++ {
					opened[yy*w+xx] = true
				}
			}
		}
	}
	return opened
}

// allSet reports whether the square of radius r around x, y lies in the image and is set.
func allSet(mask []bool, w, h, x, y, r int) bool {
	if x < r || y < r || x >= w-r || y >= h-r {
		return false
	}
	for yy := y - r; yy <= y+r; yy++ {
		for xx := x - r; xx <= x+r; xx++ {
			if !mask[yy*w+xx] {
				return false
			}
		}
	}
	return true
}

// ringPixels returns the red and green pixels of the double ring: the colored area that
// holds the tap. The thin wires between the segments are closed by growing the colored
// pixels a little, so the segments are one area, while the rest of the picture (the
// triple ring, the bull, a red-brown wall) is not connected to it. Without it the wall
// behind the board pulled the ellipse far off.
func ringPixels(mask []bool, w, h int, tap dartboard.Point, diagonal float64) []dartboard.Point {
	grow := max(2, int(math.Round(diagonal/450)))
	reach := max(10, diagonal/60) // how far from a colored pixel the tap may be

	// Grow horizontally, then vertically.
	wide := make([]bool, w*h)
	for y := range h {
		for x := range w {
			for d := -grow; d <= grow; d++ {
				if xx := x + d; xx >= 0 && xx < w && mask[y*w+xx] {
					wide[y*w+x] = true
					break
				}
			}
		}
	}
	grown := make([]bool, w*h)
	for y := range h {
		for x := range w {
			for d := -grow; d <= grow; d++ {
				if yy := y + d; yy >= 0 && yy < h && wide[yy*w+x] {
					grown[y*w+x] = true
					break
				}
			}
		}
	}

	// Start at the colored pixel nearest to the tap.
	start, nearest := -1, math.Inf(1)
	for y := range h {
		for x := range w {
			if mask[y*w+x] {
				if d := math.Hypot(float64(x)-tap.X, float64(y)-tap.Y); d < nearest {
					start, nearest = y*w+x, d
				}
			}
		}
	}
	if start < 0 || nearest > reach {
		return nil
	}

	seen := make([]bool, w*h)
	seen[start] = true
	queue := []int{start}
	var ring []dartboard.Point
	for i := 0; i < len(queue); i++ {
		q := queue[i]
		x, y := q%w, q/w
		if mask[q] {
			ring = append(ring, dartboard.Point{X: float64(x), Y: float64(y)})
		}
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			if n := ny*w + nx; !seen[n] && grown[n] {
				seen[n] = true
				queue = append(queue, n)
			}
		}
	}
	return ring
}

// ellipse is x^T M x = 1 around its center, with M = [a b; b c].
type ellipse struct {
	cx, cy  float64
	a, b, c float64
}

// radial is how far a point is from the ellipse, relative to its size.
func (e ellipse) radial(p dartboard.Point) float64 {
	x, y := p.X-e.cx, p.Y-e.cy
	return math.Sqrt(e.a*x*x+2*e.b*x*y+e.c*y*y) - 1
}

// semiAxes returns the ellipse's longer and shorter semi-axis.
func (e ellipse) semiAxes() (major, minor float64) {
	mean, diff := (e.a+e.c)/2, math.Hypot((e.a-e.c)/2, e.b)
	return 1 / math.Sqrt(mean-diff), 1 / math.Sqrt(mean+diff)
}

// plausible reports whether the ellipse can be the outer edge of the double ring in an
// image of this diagonal: not tiny, not far bigger than the image, not a sliver.
func (e ellipse) plausible(diagonal float64) bool {
	major, minor := e.semiAxes()
	return !math.IsNaN(major+minor) && major > diagonal/12 && major < diagonal && minor/major > 0.15
}

// fitEllipse fits a conic to the points by least squares, with the points centered and
// scaled first so that the equations stay well conditioned.
func fitEllipse(points []dartboard.Point) (ellipse, bool) {
	var mx, my float64
	for _, p := range points {
		mx += p.X
		my += p.Y
	}
	n := float64(len(points))
	mx, my = mx/n, my/n
	var scale float64
	for _, p := range points {
		scale += math.Hypot(p.X-mx, p.Y-my)
	}
	scale /= n
	if scale == 0 {
		return ellipse{}, false
	}

	// a x² + 2b xy + c y² + d x + e y = 1
	var ata [5][5]float64
	var atb [5]float64
	for _, p := range points {
		x, y := (p.X-mx)/scale, (p.Y-my)/scale
		row := [5]float64{x * x, 2 * x * y, y * y, x, y}
		for i := range row {
			for j := range row {
				ata[i][j] += row[i] * row[j]
			}
			atb[i] += row[i]
		}
	}
	sol, ok := solveSquare(ata, atb)
	if !ok {
		return ellipse{}, false
	}
	a, b, c, d, e := sol[0], sol[1], sol[2], sol[3], sol[4]
	det := a*c - b*b
	if det <= 0 {
		return ellipse{}, false
	}
	x0 := (-(d/2)*c + (e/2)*b) / det
	y0 := (-(e/2)*a + (d/2)*b) / det
	k := 1 + a*x0*x0 + 2*b*x0*y0 + c*y0*y0
	if k <= 0 {
		return ellipse{}, false
	}
	s2 := scale * scale
	return ellipse{cx: mx + x0*scale, cy: my + y0*scale, a: a / k / s2, b: b / k / s2, c: c / k / s2}, true
}

// solveSquare solves a 5×5 system by Gaussian elimination with pivoting.
func solveSquare(a [5][5]float64, b [5]float64) ([5]float64, bool) {
	const n = 5
	for i := range n {
		pivot := i
		for r := i + 1; r < n; r++ {
			if math.Abs(a[r][i]) > math.Abs(a[pivot][i]) {
				pivot = r
			}
		}
		a[i], a[pivot] = a[pivot], a[i]
		b[i], b[pivot] = b[pivot], b[i]
		if math.Abs(a[i][i]) < 1e-12 {
			return [5]float64{}, false
		}
		for r := i + 1; r < n; r++ {
			f := a[r][i] / a[i][i]
			for c := i; c < n; c++ {
				a[r][c] -= f * a[i][c]
			}
			b[r] -= f * b[i]
		}
	}
	var x [5]float64
	for i := n - 1; i >= 0; i-- {
		v := b[i]
		for c := i + 1; c < n; c++ {
			v -= a[i][c] * x[c]
		}
		x[i] = v / a[i][i]
	}
	return x, true
}

// outerEdgeEllipse fits the ellipse of the outer edge of the double ring to the colored
// pixels around it. Per direction from the pixels' center the edge is the outermost
// radius that still has a dense band of colored pixels just inside it, a ring and not a
// speck (a logo, a reflection), then the fit is repeated without the points that are
// farthest from it.
func outerEdgeEllipse(ring []dartboard.Point) (ellipse, bool) {
	var cx, cy float64
	for _, p := range ring {
		cx += p.X
		cy += p.Y
	}
	cx, cy = cx/float64(len(ring)), cy/float64(len(ring))
	var size float64 // about the ring's radius in pixels
	for _, p := range ring {
		size += math.Hypot(p.X-cx, p.Y-cy)
	}
	size /= float64(len(ring))

	const bins = 180
	radii := make([][]float64, bins)
	for _, p := range ring {
		bin := int((math.Atan2(p.Y-cy, p.X-cx) + math.Pi) / (2 * math.Pi) * bins)
		bin = min(bin, bins-1)
		radii[bin] = append(radii[bin], math.Hypot(p.X-cx, p.Y-cy))
	}
	// The band is 0.04 of the ring's radius wide and holds as many pixels as a ring of that
	// radius has in one bin (measured at 1280×720: 14 px, 40 pixels; the rest scales).
	band := 0.04 * size
	need := max(6, int(3e-4*size*size))
	var edge []dartboard.Point
	for bin, rs := range radii {
		sort.Float64s(rs)
		for j := len(rs) - 1; j >= need; j-- {
			if rs[j]-rs[j-need] <= band {
				angle := (float64(bin)+0.5)/bins*2*math.Pi - math.Pi
				edge = append(edge, dartboard.Point{X: cx + rs[j]*math.Cos(angle), Y: cy + rs[j]*math.Sin(angle)})
				break
			}
		}
	}
	if len(edge) < 30 {
		return ellipse{}, false
	}

	for range 6 {
		e, ok := fitEllipse(edge)
		if !ok {
			return ellipse{}, false
		}
		off := make([]float64, len(edge))
		for i, p := range edge {
			off[i] = math.Abs(e.radial(p))
		}
		sorted := append([]float64(nil), off...)
		sort.Float64s(sorted)
		limit := max(2*sorted[len(sorted)*3/4], 0.01)
		var kept []dartboard.Point
		for i, p := range edge {
			if off[i] <= limit {
				kept = append(kept, p)
			}
		}
		if len(kept) < 20 || len(kept) == len(edge) {
			break
		}
		edge = kept
	}
	return fitEllipse(edge)
}

// findBull finds the bull's eye: the small colored area nearest to the ellipse's center
// (not the center itself, see AutoCalibrate). Its size is about 0.9 % of the ring's, the
// range allows for parts that aren't seen as colored.
func findBull(mask []bool, w, h int, e ellipse) (dartboard.Point, bool) {
	major, minor := e.semiAxes()
	area := math.Pi * major * minor
	least, most := int(0.001*area), int(0.02*area)
	box := int(0.5 * minor) // the bull is up to 30 % of the minor axis away from the center
	x0, x1 := max(0, int(e.cx)-box), min(w-1, int(e.cx)+box)
	y0, y1 := max(0, int(e.cy)-box), min(h-1, int(e.cy)+box)

	seen := make([]bool, w*h)
	best, nearest := dartboard.Point{}, math.Inf(1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if !mask[y*w+x] || seen[y*w+x] {
				continue
			}
			// Flood fill the colored area, inside the box.
			seen[y*w+x] = true
			queue := []int{y*w + x}
			var sx, sy float64
			for i := 0; i < len(queue); i++ {
				qx, qy := queue[i]%w, queue[i]/w
				sx += float64(qx)
				sy += float64(qy)
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						nx, ny := qx+dx, qy+dy
						if nx < x0 || nx > x1 || ny < y0 || ny > y1 {
							continue
						}
						if n := ny*w + nx; mask[n] && !seen[n] {
							seen[n] = true
							queue = append(queue, n)
						}
					}
				}
			}
			if len(queue) < least || len(queue) > most {
				continue
			}
			c := dartboard.Point{X: sx / float64(len(queue)), Y: sy / float64(len(queue))}
			if d := math.Hypot(c.X-e.cx, c.Y-e.cy); d < nearest {
				best, nearest = c, d
			}
		}
	}
	return best, !math.IsInf(nearest, 1)
}

// polarHomography returns the homography from the image to the board in mm that maps the
// ellipse onto the double ring's outer edge and the bull's image onto the board's center,
// rotated by theta. The point whose polar line (with respect to the ellipse) is the line
// at infinity of the board plane is the board's center; a projective map that sends that
// line to infinity turns the ellipse into an ellipse whose center is the image of the
// board's center, and an affine map turns that into a circle.
func polarHomography(e ellipse, bull dartboard.Point, theta float64) (Homography, bool) {
	// The ellipse as a conic, x^T C x = 0 in homogeneous coordinates.
	d := -(e.a*e.cx + e.b*e.cy)
	f := -(e.b*e.cx + e.c*e.cy)
	g := e.a*e.cx*e.cx + 2*e.b*e.cx*e.cy + e.c*e.cy*e.cy - 1
	conic := mat3{{e.a, e.b, d}, {e.b, e.c, f}, {d, f, g}}

	// The vanishing line is the polar line of the bull; Q sends it to infinity.
	centre := [3]float64{bull.X, bull.Y, 1}
	var line [3]float64
	for i := range line {
		for j := range centre {
			line[i] += conic[i][j] * centre[j]
		}
	}
	q := mat3{{1, 0, 0}, {0, 1, 0}, line}
	qInv, ok := q.inverse()
	if !ok {
		return Homography{}, false
	}
	moved := qInv.transpose().mul(conic).mul(qInv)

	// In that chart the conic is an ellipse A x² + 2B xy + C y² + 2D x + 2E y + F = 0.
	a, b, c, dd, ee, ff := moved[0][0], moved[0][1], moved[1][1], moved[0][2], moved[1][2], moved[2][2]
	det := a*c - b*b
	if det == 0 {
		return Homography{}, false
	}
	x0 := (-dd*c + ee*b) / det
	y0 := (-ee*a + dd*b) / det
	k := -(ff + dd*x0 + ee*y0)
	a, b, c = a/k, b/k, c/k
	if a <= 0 || c-b*b/a <= 0 {
		return Homography{}, false
	}

	// U with U^T U = [a b; b c] maps the ellipse to the unit circle: the Cholesky factor.
	u11 := math.Sqrt(a)
	u12 := b / u11
	u22 := math.Sqrt(c - u12*u12)
	radius := float64(dartboard.DoubleOuterRadius)
	cos, sin := math.Cos(theta), math.Sin(theta)
	// radius · rotation · U, then the reflection: the image's y axis points down, the board's
	// up, so the second row changes sign.
	m := [2][2]float64{
		{radius * (cos * u11), radius * (cos*u12 - sin*u22)},
		{-radius * (sin * u11), -radius * (sin*u12 + cos*u22)},
	}
	to := mat3{
		{m[0][0], m[0][1], -(m[0][0]*x0 + m[0][1]*y0)},
		{m[1][0], m[1][1], -(m[1][0]*x0 + m[1][1]*y0)},
		{0, 0, 1},
	}
	h := to.mul(q)
	return Homography{h[0][0], h[0][1], h[0][2], h[1][0], h[1][1], h[1][2], h[2][0], h[2][1], h[2][2]}, true
}

type mat3 [3][3]float64

func (m mat3) mul(o mat3) (r mat3) {
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				r[i][j] += m[i][k] * o[k][j]
			}
		}
	}
	return
}

func (m mat3) transpose() (r mat3) {
	for i := range 3 {
		for j := range 3 {
			r[i][j] = m[j][i]
		}
	}
	return
}

func (m mat3) inverse() (mat3, bool) {
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) - m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) + m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	if det == 0 || math.IsNaN(det) {
		return mat3{}, false
	}
	return mat3{
		{(m[1][1]*m[2][2] - m[1][2]*m[2][1]) / det, (m[0][2]*m[2][1] - m[0][1]*m[2][2]) / det, (m[0][1]*m[1][2] - m[0][2]*m[1][1]) / det},
		{(m[1][2]*m[2][0] - m[1][0]*m[2][2]) / det, (m[0][0]*m[2][2] - m[0][2]*m[2][0]) / det, (m[0][2]*m[1][0] - m[0][0]*m[1][2]) / det},
		{(m[1][0]*m[2][1] - m[1][1]*m[2][0]) / det, (m[0][1]*m[2][0] - m[0][0]*m[2][1]) / det, (m[0][0]*m[1][1] - m[0][1]*m[1][0]) / det},
	}, true
}
