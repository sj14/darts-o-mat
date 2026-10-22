package dartboard

import (
	"testing"
)

func TestScore(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    Point
		want Hit
	}{
		{"bullseye", Point{0, 0}, Hit{Number: BullNumber, Segment: Bullseye}},
		{"bull", Point{10, 0}, Hit{Number: BullNumber, Segment: Bull}},
		{"T20", Polar(103, 0), Hit{Number: 20, Segment: Triple}},
		{"inner single 20", Polar(50, 0), Hit{Number: 20, Segment: Inner}},
		{"outer single 6", Polar(130, 90), Hit{Number: 6, Segment: Outer}},
		{"D3", Polar(166, 180), Hit{Number: 3, Segment: Double}},
		{"T11", Polar(103, 270), Hit{Number: 11, Segment: Triple}},
		{"5 just left of the 20", Polar(50, 350), Hit{Number: 5, Segment: Inner}},
		{"1 just right of the 20", Polar(50, 10), Hit{Number: 1, Segment: Inner}},
		{"outside, beside the 7", Polar(175, 216), Hit{Segment: Miss, Number: 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.p); got != tt.want {
				t.Errorf("Score(%v) = %+v, want %+v", tt.p, got, tt.want)
			}
		})
	}
}
