package vision

import (
	"math"
	"testing"

	"github.com/sj14/darts-o-mat/common/dartboard"
)

func TestEstimateHomography(t *testing.T) {
	// A known perspective mapping, similar to a camera looking at the board at an angle.
	want := Homography{2.1, 0.3, 640, -0.2, 1.2, 360, 0.0004, 0.0011, 1}

	var board, image []dartboard.Point
	for _, angle := range []float64{9, 99, 189, 279, 45, 135} {
		p := dartboard.Polar(dartboard.DoubleOuterRadius, angle)
		board = append(board, p)
		image = append(image, want.Apply(p))
	}

	h, err := EstimateHomography(board, image)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []dartboard.Point{{X: 0, Y: 0}, dartboard.Polar(103, 0), dartboard.Polar(166, 200)} {
		got, exp := h.Apply(p), want.Apply(p)
		if math.Hypot(got.X-exp.X, got.Y-exp.Y) > 1e-6 {
			t.Errorf("Apply(%v) = %v, want %v", p, got, exp)
		}
	}

	inv, err := h.Inverse()
	if err != nil {
		t.Fatal(err)
	}
	p := dartboard.Polar(50, 123)
	if back := inv.Apply(h.Apply(p)); math.Hypot(back.X-p.X, back.Y-p.Y) > 1e-6 {
		t.Errorf("inverse: got %v back, want %v", back, p)
	}
}

func TestEstimateHomographyDegenerate(t *testing.T) {
	line := []dartboard.Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 2, Y: 2}, {X: 3, Y: 3}}
	if _, err := EstimateHomography(line, line); err == nil {
		t.Error("want an error for points on one line")
	}
}
