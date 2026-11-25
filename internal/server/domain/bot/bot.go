// Package bot throws the darts of a bot: it aims at the middle of a field (game.Game.Aim)
// and its dart lands around it, spread like a player's.
package bot

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// The spread of a bot's darts (see Throw): the smallest, the largest and the one a new bot
// gets.
const (
	MinSpread     game.Spread = 40  // 4 mm
	MaxSpread     game.Spread = 800 // 80 mm
	DefaultSpread game.Spread = 200 // 20 mm
)

// SliderSteps is how many steps the slider that chooses a spread has, from 0 (MaxSpread, the
// weakest) to SliderSteps (MinSpread). A step is 0.3 % of the spread: up to about 36 mm every
// tenth of a mm can be chosen, above that a step is 0.2 mm or 0.3 mm.
const SliderSteps = 1000

// SpreadAt returns the spread at a position of the slider, which runs from MaxSpread (0, on
// the left) to MinSpread (SliderSteps, on the right) on a log scale: the same distance is
// the same relative change, as a bot's strength changes with it. The spread is rounded to a
// tenth of a mm. Outside the slider the nearest end.
func SpreadAt(position int) game.Spread {
	share := float64(min(max(position, 0), SliderSteps)) / SliderSteps
	return game.Spread(math.Round(float64(MaxSpread) * math.Pow(float64(MinSpread)/float64(MaxSpread), share)))
}

// Position returns the position of the slider closest to the spread, not an exact inverse of
// SpreadAt: a spread the slider can't choose (above about 36 mm) gets the nearest one.
func Position(spread game.Spread) int {
	spread = min(max(spread, MinSpread), MaxSpread)
	return int(math.Round(SliderSteps * math.Log(float64(MaxSpread)/float64(spread)) / math.Log(float64(MaxSpread)/float64(MinSpread))))
}

// CheckSpread checks that a bot's spread is between MinSpread and MaxSpread.
func CheckSpread(spread game.Spread) error {
	if spread < MinSpread || spread > MaxSpread {
		return fmt.Errorf("a bot's spread must be between %v and %v mm", MinSpread.MM(), MaxSpread.MM())
	}
	return nil
}

// Throw returns where a bot's dart lands that aims at the field: around the middle of
// it, off by a normal distribution with the spread (the standard deviation) in each
// direction. The dart has its position, like one the cameras saw.
func Throw(r *rand.Rand, aim throw.Dart, spread game.Spread) throw.Dart {
	p := middle(aim)
	p.X += r.NormFloat64() * spread.MM()
	p.Y += r.NormFloat64() * spread.MM()
	hit := dartboard.Score(p)
	hit.X, hit.Y = &p.X, &p.Y
	return throw.FromHit(hit)
}

// middle returns the middle of a field: the bull's for both bulls, and for a single the
// outer one's, the larger.
func middle(d throw.Dart) dartboard.Point {
	var radius float64
	switch d.Segment {
	case dartboard.Bull, dartboard.Bullseye:
		return dartboard.Point{}
	case dartboard.Triple:
		radius = (dartboard.TripleInnerRadius + dartboard.TripleOuterRadius) / 2
	case dartboard.Double:
		radius = (dartboard.DoubleInnerRadius + dartboard.DoubleOuterRadius) / 2
	case dartboard.Inner:
		radius = (dartboard.BullRadius + dartboard.TripleInnerRadius) / 2
	default:
		radius = (dartboard.TripleOuterRadius + dartboard.DoubleInnerRadius) / 2
	}
	return dartboard.Polar(radius, float64(dartboard.SegmentIndex(d.Number))*dartboard.SegmentAngle)
}

// strengthSpreads are the spreads of strengths, in tenths of a mm.
var strengthSpreads = []game.Spread{40, 50, 60, 70, 80, 90, 100, 110, 120, 130, 150, 200, 250, 300, 400, 500, 600, 800}

// strengths is what a bot plays at each of strengthSpreads, by mode: the 3-dart average
// in X01 (501 double out), the share of hits in around the clock, the marks per round in
// cricket; worked out by TestStrengths (legs alone per spread, at most 200 darts each),
// which also checks them.
var strengths = map[game.Mode][]float64{
	game.ModeX01:            {125.49, 112.97, 102.56, 94.62, 87.52, 83.29, 75.23, 70.27, 65.51, 61.4, 54.76, 41.53, 34.93, 26.82, 21.95, 16.74, 12.78, 11.18},
	game.ModeAroundTheClock: {1, 1, 1, 1, 0.99, 0.98, 0.96, 0.95, 0.92, 0.89, 0.83, 0.67, 0.54, 0.44, 0.3, 0.22, 0.16, 0.1},
	game.ModeCricket:        {5.92, 5.39, 4.79, 4.42, 4.06, 3.73, 3.44, 3.19, 3, 2.75, 2.38, 1.78, 1.39, 1.1, 0.76, 0.62, 0.48, 0.36},
}

// Strength returns about what a bot with the spread plays in the mode: the 3-dart average
// in X01, the share of hits in around the clock (0 to 1), the marks per round in cricket,
// as the players' statistics count them. It is interpolated between the spreads measured.
func Strength(mode game.Mode, spread game.Spread) float64 {
	values := strengths[mode]
	if len(values) == 0 {
		return 0
	}
	if spread <= strengthSpreads[0] {
		return values[0]
	}
	for i := 1; i < len(strengthSpreads); i++ {
		if spread <= strengthSpreads[i] {
			lo, hi := strengthSpreads[i-1], strengthSpreads[i]
			share := float64(spread-lo) / float64(hi-lo)
			return values[i-1] + share*(values[i]-values[i-1])
		}
	}
	return values[len(values)-1]
}
