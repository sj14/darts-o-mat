package dartboard

import (
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name string
		p    Point
		want throw.Dart
	}{
		{"bullseye", Point{0, 0}, throw.Dart{Number: throw.Bull, Multiplier: 2}},
		{"bull", Point{10, 0}, throw.Dart{Number: throw.Bull, Multiplier: 1}},
		{"T20", Polar(103, 0), throw.Dart{Number: 20, Multiplier: 3}},
		{"inner single 20", Polar(50, 0), throw.Dart{Number: 20, Multiplier: 1}},
		{"outer single 6", Polar(130, 90), throw.Dart{Number: 6, Multiplier: 1}},
		{"D3", Polar(166, 180), throw.Dart{Number: 3, Multiplier: 2}},
		{"T11", Polar(103, 270), throw.Dart{Number: 11, Multiplier: 3}},
		{"5 just left of the 20", Polar(50, 350), throw.Dart{Number: 5, Multiplier: 1}},
		{"1 just right of the 20", Polar(50, 10), throw.Dart{Number: 1, Multiplier: 1}},
		{"outside, beside the 7", Polar(175, 216), throw.Dart{Miss: true, Number: 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Score(tt.p); got != tt.want {
				t.Errorf("Score(%v) = %+v, want %+v", tt.p, got, tt.want)
			}
		})
	}
}
