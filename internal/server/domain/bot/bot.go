// Package bot throws the darts of a bot: it aims at the middle of a field (game.Game.Aim)
// and its dart lands around it, spread like a player's.
package bot

import (
	"fmt"
	"math/rand/v2"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// The spread of a bot's darts in mm (see Throw): the smallest, the largest and the one a
// new bot gets.
const (
	MinSpread     = 1
	MaxSpread     = 80
	DefaultSpread = 20
)

// CheckSpread checks that a bot's spread is between MinSpread and MaxSpread.
func CheckSpread(spread int) error {
	if spread < MinSpread || spread > MaxSpread {
		return fmt.Errorf("a bot's spread must be between %d and %d mm", MinSpread, MaxSpread)
	}
	return nil
}

// Throw returns where a bot's dart lands that aims at the field: around the middle of
// it, off by a normal distribution with the spread (the standard deviation in mm) in
// each direction. The dart has its position, like one the cameras saw.
func Throw(r *rand.Rand, aim throw.Dart, spread int) throw.Dart {
	p := middle(aim)
	p.X += r.NormFloat64() * float64(spread)
	p.Y += r.NormFloat64() * float64(spread)
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

// strengthSpreads are the spreads of strengths, in mm.
var strengthSpreads = []int{4, 6, 8, 10, 12, 15, 20, 25, 30, 40, 50, 60, 80}

// strengths is what a bot plays at each of strengthSpreads, by mode: the 3-dart average
// in X01 (501 double out), the share of hits in around the clock, the marks per round in
// cricket; worked out by TestStrengths (50 legs alone per spread, at most 200 darts each),
// which also checks them.
var strengths = map[game.Mode][]float64{
	game.ModeX01:            {128, 103, 90, 76, 63, 55, 41, 35, 29, 23, 17, 13, 11},
	game.ModeAroundTheClock: {1, 1, 0.99, 0.97, 0.92, 0.84, 0.67, 0.54, 0.43, 0.29, 0.21, 0.16, 0.1},
	game.ModeCricket:        {5.8, 4.8, 4.2, 3.5, 2.9, 2.4, 1.7, 1.5, 1.1, 0.75, 0.57, 0.5, 0.38},
}

// Strength returns about what a bot with the spread plays in the mode: the 3-dart average
// in X01, the share of hits in around the clock (0 to 1), the marks per round in cricket,
// as the players' statistics count them. It is interpolated between the spreads measured.
func Strength(mode game.Mode, spread int) float64 {
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
