package x01

import (
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func TestCheckout(t *testing.T) {
	bull := throw.Dart{Number: throw.Bull, Multiplier: 2}
	tests := []struct {
		mode      OutMode
		remaining uint64
		darts     int
		want      []throw.Dart
	}{
		{OutModeDouble, 170, 3, []throw.Dart{hit(20, 3), hit(20, 3), bull}},
		{OutModeDouble, 100, 3, []throw.Dart{hit(20, 3), hit(20, 2)}},
		{OutModeDouble, 60, 2, []throw.Dart{hit(20, 1), hit(20, 2)}},
		// The routes of the checkout table, not the shortest or most obvious ones.
		{OutModeDouble, 90, 3, []throw.Dart{hit(20, 3), hit(15, 2)}},
		{OutModeDouble, 99, 3, []throw.Dart{hit(19, 3), hit(10, 1), hit(16, 2)}},
		{OutModeDouble, 82, 3, []throw.Dart{bull, hit(16, 2)}},
		{OutModeDouble, 101, 3, []throw.Dart{hit(20, 3), hit(9, 1), hit(16, 2)}},
		{OutModeDouble, 50, 3, []throw.Dart{hit(10, 1), hit(20, 2)}},
		// With two darts left, the table's three-dart route doesn't fit.
		{OutModeDouble, 101, 2, []throw.Dart{hit(17, 3), bull}},
		// Odd scores below 41: a single that leaves 32, 16, 8, ...
		{OutModeDouble, 39, 3, []throw.Dart{hit(7, 1), hit(16, 2)}},
		{OutModeDouble, 31, 3, []throw.Dart{hit(15, 1), hit(8, 2)}},
		{OutModeDouble, 3, 2, []throw.Dart{hit(1, 1), hit(1, 2)}},
		{OutModeDouble, 32, 1, []throw.Dart{hit(16, 2)}},
		{OutModeDouble, 50, 1, []throw.Dart{bull}},
		{OutModeDouble, 41, 1, nil},
		{OutModeDouble, 100, 1, nil},
		{OutModeDouble, 169, 3, nil}, // a bogey number
		{OutModeDouble, 171, 3, nil},
		{OutModeDouble, 1, 3, nil},
		{OutModeSingle, 60, 1, []throw.Dart{hit(20, 3)}},
		{OutModeSingle, 20, 1, []throw.Dart{hit(20, 1)}},
		{OutModeMaster, 60, 1, []throw.Dart{hit(20, 3)}},
		{OutModeMaster, 40, 1, []throw.Dart{hit(20, 2)}},
	}
	for _, tt := range tests {
		if got := tt.mode.Checkout(tt.remaining, tt.darts); !slices.Equal(got, tt.want) {
			t.Errorf("%v %d with %d darts: got %v, want %v", tt.mode, tt.remaining, tt.darts, got, tt.want)
		}
	}
}

// TestCheckoutValid checks every suggestion against the rules by playing it.
func TestCheckoutValid(t *testing.T) {
	for _, mode := range []OutMode{OutModeSingle, OutModeDouble, OutModeMaster} {
		for remaining := uint64(1); remaining <= 180; remaining++ {
			for darts := 1; darts <= 3; darts++ {
				path := mode.Checkout(remaining, darts)
				if path == nil {
					continue
				}
				if len(path) > darts {
					t.Fatalf("%v %d: %d darts suggested, %d left", mode, remaining, len(path), darts)
				}
				st, err := Play(Settings{StartScore: remaining, OutMode: mode}, []int64{1}, path)
				if err != nil || !st.GameOver {
					t.Fatalf("%v %d: suggestion %v doesn't finish: %v", mode, remaining, path, err)
				}
			}
		}
	}
}
