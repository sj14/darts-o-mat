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

// The spread of a bot's darts in µm (see Throw): the smallest, the largest and the one a
// new bot gets.
const (
	MinSpreadUM     = 4_000
	MaxSpreadUM     = 80_000
	DefaultSpreadUM = 20_000
)

// SliderSteps is how many steps the slider that chooses a spread has, from 0 (MaxSpreadUM,
// the weakest) to SliderSteps (MinSpreadUM). A step is 0.3 % of the spread: up to about
// 36 mm every tenth of a mm can be chosen, above that a step is 0.2 mm or 0.3 mm.
const SliderSteps = 1000

// sliderRoundingUM is what the slider rounds a spread to: a tenth of a mm, finer changes
// nothing a player would notice.
const sliderRoundingUM = 100

// SpreadAt returns the spread in µm at a position of the slider, which runs from
// MaxSpreadUM (0, on the left) to MinSpreadUM (SliderSteps, on the right) on a log scale:
// the same distance is the same relative change, as a bot's strength changes with it. The
// spread is rounded to a tenth of a mm. Outside the slider the nearest end.
func SpreadAt(position int) int {
	share := float64(min(max(position, 0), SliderSteps)) / SliderSteps
	spreadUM := MaxSpreadUM * math.Pow(float64(MinSpreadUM)/MaxSpreadUM, share)
	return int(math.Round(spreadUM/sliderRoundingUM)) * sliderRoundingUM
}

// Position returns the position of the slider closest to the spread in µm, not an exact
// inverse of SpreadAt: a spread the slider can't choose (above about 36 mm) gets the
// nearest one.
func Position(spreadUM int) int {
	spreadUM = min(max(spreadUM, MinSpreadUM), MaxSpreadUM)
	share := math.Log(MaxSpreadUM/float64(spreadUM)) / math.Log(float64(MaxSpreadUM)/MinSpreadUM)
	return int(math.Round(SliderSteps * share))
}

// CheckSpread checks that a bot's spread in µm is between MinSpreadUM and MaxSpreadUM.
func CheckSpread(spreadUM int) error {
	if spreadUM < MinSpreadUM || spreadUM > MaxSpreadUM {
		return fmt.Errorf("a bot's spread must be between %d and %d µm", MinSpreadUM, MaxSpreadUM)
	}
	return nil
}

// Throw returns where a bot's dart lands that aims at the field: around the middle of
// it, off by a normal distribution with the spread (the standard deviation in µm) in
// each direction. The dart has its position, like one the cameras saw.
func Throw(r *rand.Rand, aim throw.Dart, spreadUM int) throw.Dart {
	spreadMM := float64(spreadUM) / 1000
	p := middle(aim)
	p.X += r.NormFloat64() * spreadMM
	p.Y += r.NormFloat64() * spreadMM
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

// strengthSpreads are the spreads of strengths, in µm, from the strongest to the weakest.
var strengthSpreads = []int{
	4_000, 5_000, 6_000, 7_000, 8_000, 9_000, 10_000, 11_000, 12_000, 13_000, 15_000,
	20_000, 25_000, 30_000, 40_000, 50_000, 60_000, 80_000,
}

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
func Strength(mode game.Mode, spreadUM int) float64 {
	values := strengths[mode]
	if len(values) == 0 {
		return 0
	}
	if spreadUM <= strengthSpreads[0] {
		return values[0]
	}
	for i := 1; i < len(strengthSpreads); i++ {
		if spreadUM <= strengthSpreads[i] {
			lo, hi := strengthSpreads[i-1], strengthSpreads[i]
			share := float64(spreadUM-lo) / float64(hi-lo)
			return values[i-1] + share*(values[i]-values[i-1])
		}
	}
	return values[len(values)-1]
}
