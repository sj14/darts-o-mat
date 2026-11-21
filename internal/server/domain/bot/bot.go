// Package bot throws the darts of a bot: it aims at the middle of a field (game.Game.Aim)
// and its dart lands around it, spread like a player's.
package bot

import (
	"fmt"
	"math/rand/v2"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
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
