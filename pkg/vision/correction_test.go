package vision

import (
	"math"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// correctionOf returns a correction where every ring lies off its radius by offset(ring, angle).
func correctionOf(offset func(ring, angle float64) float64) *GridCorrection {
	c := &GridCorrection{}
	for _, r := range allRingRadii {
		ring := make([]float64, 360/correctionStep)
		for k := range ring {
			ring[k] = offset(r, float64(k*correctionStep))
		}
		c.Radii = append(c.Radii, r)
		c.Offsets = append(c.Offsets, ring)
	}
	return c
}

func TestGridCorrection(t *testing.T) {
	// The double ring is found 1 mm outside its wires towards the 6 (90°), on the other
	// side not at all; all other rings are where they belong.
	c := correctionOf(func(ring, angle float64) float64 {
		if ring >= dartboard.DoubleInnerRadius {
			return math.Max(0, math.Cos((angle-90)*math.Pi/180))
		}
		return 0
	})
	tests := []struct {
		name     string
		in, want dartboard.Point
	}{
		{"on the double's inner wire", dartboard.Polar(163, 90), dartboard.Polar(162, 90)},
		{"outside the board, moved like the outer ring", dartboard.Polar(200, 90), dartboard.Polar(199, 90)},
		{"between triple and double, in proportion", dartboard.Polar(107+56/2.0, 90), dartboard.Polar(107+55/2.0, 90)},
		{"the triple stays", dartboard.Polar(103, 90), dartboard.Polar(103, 90)},
		{"opposite side stays", dartboard.Polar(165, 270), dartboard.Polar(165, 270)},
		{"halfway around: half the offset", dartboard.Polar(162.5, 30), dartboard.Polar(162, 30)},
		{"center", dartboard.Point{}, dartboard.Point{}},
	}
	for _, tt := range tests {
		got := c.toBoard(tt.in)
		if dist(got, tt.want) > 0.01 {
			t.Errorf("%s: %.2f mm at %.1f°, want %.2f mm", tt.name, got.Radius(), got.Angle(), tt.want.Radius())
		}
		if back := c.fromBoard(got); dist(back, tt.in) > 0.01 {
			t.Errorf("%s: back to %.2f,%.2f, want %.2f,%.2f", tt.name, back.X, back.Y, tt.in.X, tt.in.Y)
		}
	}

	var none *GridCorrection
	if p := dartboard.Polar(163, 90); none.toBoard(p) != p || none.fromBoard(p) != p {
		t.Error("no correction must leave points alone")
	}
}

func TestMappingTip(t *testing.T) {
	// The image is the board, 1 px per mm, but upside down: the image's y points down.
	m := Mapping{ToBoard: Homography{1, 0, 0, 0, -1, 0, 0, 0, 1}}
	// A dart in the 20 whose flight points up in the image, away from the center.
	d := Detection{Tip: dartboard.Point{X: 0, Y: -100}, Axis: dartboard.Point{X: 0, Y: -1}}
	if got := m.Tip(d, 0); dist(got, dartboard.Point{Y: 100}) > 1e-9 {
		t.Errorf("without shift: %+v", got)
	}
	if got := m.Tip(d, 1.5); dist(got, dartboard.Point{Y: 101.5}) > 1e-9 {
		t.Errorf("moved back 1.5 mm along the dart: %+v, want 0,101.5", got)
	}
}
