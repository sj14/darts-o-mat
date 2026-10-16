// Package vision contains the camera geometry that doesn't need OpenCV:
// mapping between camera images and the dartboard plane.
package vision

import (
	"errors"
	"math"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// Homography maps points of one plane onto another, here between a camera image
// (pixels) and the dartboard (mm). It is a row-major 3x3 matrix.
type Homography [9]float64

var ErrDegenerate = errors.New("points are degenerate, e.g. three on one line")

// EstimateHomography returns the homography mapping each src point onto its dst point.
// It needs at least 4 pairs; with more it finds the least-squares fit.
func EstimateHomography(src, dst []dartboard.Point) (Homography, error) {
	if len(src) != len(dst) || len(src) < 4 {
		return Homography{}, errors.New("need at least 4 point pairs")
	}

	// Normalizing both point sets keeps the equations well conditioned (Hartley).
	srcN, srcT := normalize(src)
	dstN, dstT := normalize(dst)

	// Each pair gives two equations for the 8 unknowns h0..h7 (h8 = 1).
	var ata [8][8]float64
	var atb [8]float64
	for i := range srcN {
		x, y := srcN[i].X, srcN[i].Y
		u, v := dstN[i].X, dstN[i].Y
		rows := [2][8]float64{
			{x, y, 1, 0, 0, 0, -u * x, -u * y},
			{0, 0, 0, x, y, 1, -v * x, -v * y},
		}
		rhs := [2]float64{u, v}
		for r := range rows {
			for j := range 8 {
				atb[j] += rows[r][j] * rhs[r]
				for k := range 8 {
					ata[j][k] += rows[r][j] * rows[r][k]
				}
			}
		}
	}

	h, err := solve(ata, atb)
	if err != nil {
		return Homography{}, err
	}
	hn := Homography{h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7], 1}

	// Undo the normalization: H = dstT⁻¹ · Hn · srcT.
	dstInv, err := dstT.Inverse()
	if err != nil {
		return Homography{}, err
	}
	return dstInv.Mul(hn).Mul(srcT).scaled(), nil
}

// Apply maps p with the homography.
func (h Homography) Apply(p dartboard.Point) dartboard.Point {
	w := h[6]*p.X + h[7]*p.Y + h[8]
	return dartboard.Point{
		X: (h[0]*p.X + h[1]*p.Y + h[2]) / w,
		Y: (h[3]*p.X + h[4]*p.Y + h[5]) / w,
	}
}

func (h Homography) Mul(o Homography) Homography {
	var r Homography
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				r[i*3+j] += h[i*3+k] * o[k*3+j]
			}
		}
	}
	return r
}

// Inverse returns the homography mapping the other way round.
func (h Homography) Inverse() (Homography, error) {
	a, b, c := h[0], h[1], h[2]
	d, e, f := h[3], h[4], h[5]
	g, k, l := h[6], h[7], h[8]
	det := a*(e*l-f*k) - b*(d*l-f*g) + c*(d*k-e*g)
	if math.Abs(det) < 1e-12 {
		return Homography{}, ErrDegenerate
	}
	inv := Homography{
		e*l - f*k, c*k - b*l, b*f - c*e,
		f*g - d*l, a*l - c*g, c*d - a*f,
		d*k - e*g, b*g - a*k, a*e - b*d,
	}
	for i := range inv {
		inv[i] /= det
	}
	return inv.scaled(), nil
}

// scaled divides by the last element, which doesn't change the mapping.
func (h Homography) scaled() Homography {
	if h[8] == 0 {
		return h
	}
	s := h[8]
	for i := range h {
		h[i] /= s
	}
	return h
}

// normalize moves the points' centroid to the origin and scales them to an
// average distance of √2. It returns the points and the transformation used.
func normalize(points []dartboard.Point) ([]dartboard.Point, Homography) {
	var cx, cy float64
	for _, p := range points {
		cx += p.X
		cy += p.Y
	}
	cx /= float64(len(points))
	cy /= float64(len(points))

	var dist float64
	for _, p := range points {
		dist += math.Hypot(p.X-cx, p.Y-cy)
	}
	dist /= float64(len(points))
	s := math.Sqrt2
	if dist > 0 {
		s /= dist
	}

	t := Homography{s, 0, -s * cx, 0, s, -s * cy, 0, 0, 1}
	result := make([]dartboard.Point, len(points))
	for i, p := range points {
		result[i] = t.Apply(p)
	}
	return result, t
}

// solve solves the linear system with Gaussian elimination and partial pivoting.
func solve(a [8][8]float64, b [8]float64) ([8]float64, error) {
	const n = 8
	for col := range n {
		pivot := col
		for r := col + 1; r < n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[pivot][col]) {
				pivot = r
			}
		}
		if math.Abs(a[pivot][col]) < 1e-12 {
			return [8]float64{}, ErrDegenerate
		}
		a[col], a[pivot] = a[pivot], a[col]
		b[col], b[pivot] = b[pivot], b[col]

		for r := col + 1; r < n; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c < n; c++ {
				a[r][c] -= f * a[col][c]
			}
			b[r] -= f * b[col]
		}
	}

	var x [8]float64
	for r := n - 1; r >= 0; r-- {
		sum := b[r]
		for c := r + 1; c < n; c++ {
			sum -= a[r][c] * x[c]
		}
		x[r] = sum / a[r][r]
	}
	return x, nil
}
