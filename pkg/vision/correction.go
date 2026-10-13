package vision

import (
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// Mapping maps a camera's image to the board.
type Mapping struct {
	ToBoard Homography
	// Correction is nil if there is none, e.g. for a calibration from clicked points only.
	Correction *GridCorrection
}

// Apply maps an image point (pixels) to the board (mm).
func (m Mapping) Apply(p dartboard.Point) dartboard.Point {
	return m.Correction.toBoard(m.ToBoard.Apply(p))
}

// ToImage maps a board point (mm) into the image (pixels).
func (m Mapping) ToImage(p dartboard.Point) (dartboard.Point, error) {
	toImage, err := m.ToBoard.Inverse()
	if err != nil {
		return dartboard.Point{}, err
	}
	return toImage.Apply(m.Correction.fromBoard(p)), nil
}

// Tip maps a dart's tip found in the image to the board, moved back along the dart by
// shift mm: the change in the image tends to reach a little beyond the real tip (see
// DetectOptions.TipShift).
func (m Mapping) Tip(d Detection, shift float64) dartboard.Point {
	p := m.Apply(d.Tip)
	if shift == 0 {
		return p
	}
	// The dart's axis on the board, towards the flight.
	q := m.Apply(dartboard.Point{X: d.Tip.X + 10*d.Axis.X, Y: d.Tip.Y + 10*d.Axis.Y})
	n := math.Hypot(q.X-p.X, q.Y-p.Y)
	if n == 0 {
		return p
	}
	return dartboard.Point{X: p.X + shift*(q.X-p.X)/n, Y: p.Y + shift*(q.Y-p.Y)/n}
}

// GridCorrection moves points mapped by a camera's homography onto the board's real
// rings. The fine calibration fits one homography to the whole board, but in places a
// ring still lies up to about 1 mm off it (2026-10-13: the same off at the inner and outer
// wire of the double, differently per camera, so neither noise nor the board itself).
// It is radial only: the wires between the numbers are found too rarely to correct them.
type GridCorrection struct {
	// Radii are the rings' radii (mm), ascending. Offsets[i][k] is how far ring i lies
	// outside its radius at the angle k·360°/len(Offsets[i]), in mm.
	Radii   []float64   `json:"radii"`
	Offsets [][]float64 `json:"offsets"`
}

// correctionStep is the angle between two offsets of a ring (degrees), correctionWidth
// how far around it the edges found count, less the farther they are (degrees).
const (
	correctionStep  = 5
	correctionWidth = 15
)

// newGridCorrection measures how far each ring's edges were found off the homography,
// smoothed around the ring. feats are the ring edges used for the fit; their board
// positions are where the edges are expected, ring is the radius of their wire.
func newGridCorrection(toBoard Homography, feats []ringEdge) *GridCorrection {
	byRing := map[float64][]ringEdge{}
	for _, e := range feats {
		byRing[e.ring] = append(byRing[e.ring], e)
	}
	c := &GridCorrection{}
	for _, r := range allRingRadii {
		edges := byRing[r]
		if len(edges) < 6 {
			continue // e.g. the bull hidden by darts: no correction there
		}
		angles := make([]float64, len(edges))
		offsets := make([]float64, len(edges))
		for i, e := range edges {
			p := toBoard.Apply(e.found)
			angles[i] = e.board.Angle()
			offsets[i] = p.Radius() - e.board.Radius()
		}
		median := slices.Clone(offsets)
		slices.Sort(median)
		ring := make([]float64, 360/correctionStep)
		for k := range ring {
			var sum, weights float64
			for i, a := range angles {
				d := math.Abs(math.Remainder(a-float64(k*correctionStep), 360))
				if w := 1 - d/correctionWidth; w > 0 {
					sum += w * offsets[i]
					weights += w
				}
			}
			ring[k] = median[len(median)/2] // too few edges around: the ring's typical offset
			if weights >= 1.5 {
				ring[k] = sum / weights
			}
		}
		c.Radii = append(c.Radii, r)
		c.Offsets = append(c.Offsets, ring)
	}
	if len(c.Radii) == 0 {
		return nil
	}
	return c
}

// ringEdge is an edge of a ring found in the image (pixels) and where on the board it is
// expected (mm); ring is the radius of its wire.
type ringEdge struct {
	board, found dartboard.Point
	ring         float64
}

// offset returns how far ring i lies outside its radius at an angle (degrees).
func (c *GridCorrection) offset(i int, angle float64) float64 {
	o := c.Offsets[i]
	pos := math.Mod(angle, 360) / 360 * float64(len(o))
	k := int(pos) % len(o)
	t := pos - math.Floor(pos)
	return o[k]*(1-t) + o[(k+1)%len(o)]*t
}

// toBoard moves a point mapped by the homography to where it is relative to the real
// rings: between two rings in proportion, inside the innermost towards the center,
// outside the outermost by its offset.
func (c *GridCorrection) toBoard(p dartboard.Point) dartboard.Point {
	if c == nil {
		return p
	}
	return c.radial(p, false)
}

// fromBoard is the inverse of toBoard: where the homography puts a board point.
func (c *GridCorrection) fromBoard(p dartboard.Point) dartboard.Point {
	if c == nil {
		return p
	}
	return c.radial(p, true)
}

func (c *GridCorrection) radial(p dartboard.Point, inverse bool) dartboard.Point {
	r, a := p.Radius(), p.Angle()
	// Piecewise linear through the rings: from where they are found to their radius.
	from := make([]float64, len(c.Radii))
	to := make([]float64, len(c.Radii))
	for i, radius := range c.Radii {
		from[i], to[i] = radius+c.offset(i, a), radius
	}
	if inverse {
		from, to = to, from
	}
	var corrected float64
	switch last := len(from) - 1; {
	case r <= from[0]:
		corrected = r * to[0] / from[0]
	case r >= from[last]:
		corrected = r - from[last] + to[last]
	default:
		i := 0
		for r > from[i+1] {
			i++
		}
		t := (r - from[i]) / (from[i+1] - from[i])
		corrected = to[i] + t*(to[i+1]-to[i])
	}
	return dartboard.Polar(corrected, a)
}
