package turn

import (
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// bullOff decides who starts the first leg: each contender throws one dart at the
// bull, the closest starts. Equally close players throw again, unless the players
// decided who was closer. Who takes too long for their dart (skip) is out.
type bullOff struct {
	contenders []int // players still bulling off, in throwing order; nil once decided
	closer     *int  // see Log.BullOffCloser
	round      int   // 0 for everyone's first dart, then the ties
	best       float64
	closest    []int // the players with the best distance in this round
	throws     []bullOffThrow
}

// bullOffThrow is a dart thrown to bull off.
type bullOffThrow struct {
	Entry  int // index in the log
	Player int
	Round  int // 0 for everyone's first dart, then 1, 2, … for the ties
}

func newBullOff(players int, closer *int) *bullOff {
	b := &bullOff{closer: closer, best: math.Inf(1)}
	for i := range players {
		b.contenders = append(b.contenders, i)
	}
	return b
}

// throw counts the bull-off dart of player. It returns who throws next, or the
// starter of the first leg once that is decided.
func (b *bullOff) throw(entry, player int, d throw.Dart) (next int, decided bool) {
	b.throws = append(b.throws, bullOffThrow{Entry: entry, Player: player, Round: b.round})
	switch dist := bullDistance(d); {
	case dist < b.best:
		b.best, b.closest = dist, []int{player}
	case dist == b.best:
		b.closest = append(b.closest, player)
	}
	if i := slices.Index(b.contenders, player); i+1 < len(b.contenders) {
		return b.contenders[i+1], false
	}
	return b.endRound()
}

// skip takes the player out who took too long for their dart (Turn.Skip): they can't be
// the closest anymore and don't throw again after a tie. It returns who throws next, or the
// starter of the first leg once that is decided: the only one left starts at once.
func (b *bullOff) skip(player int) (next int, decided bool) {
	i := slices.Index(b.contenders, player)
	b.contenders = slices.Delete(b.contenders, i, i+1)
	switch {
	case len(b.contenders) == 1:
		starter := b.contenders[0]
		b.contenders = nil
		return starter, true
	case i < len(b.contenders):
		return b.contenders[i], false
	}
	return b.endRound()
}

// endRound decides once everyone in the round threw or was skipped (someone threw, else
// only one would be left, see skip): the closest starts, those equally close throw again.
func (b *bullOff) endRound() (next int, decided bool) {
	if b.closer != nil && slices.Contains(b.closest, *b.closer) {
		b.closest = []int{*b.closer}
	}
	if len(b.closest) == 1 {
		b.contenders = nil
		return b.closest[0], true
	}
	b.contenders, b.closest, b.best = b.closest, nil, math.Inf(1)
	b.round++
	return b.contenders[0], false
}

// singleBull is where a single bull without a position counts, in mm from the center.
const singleBull = 11

// bullDistance is how far from the center of the bull a dart hit, in mm: from the
// position the cameras saw; without it 0 for the bullseye, the middle of the bull for
// a single bull, infinite otherwise.
func bullDistance(d throw.Dart) float64 {
	switch {
	case d.X != nil && d.Y != nil:
		return math.Hypot(*d.X, *d.Y)
	case d.Segment == dartboard.Bullseye:
		return 0
	case d.Segment == dartboard.Bull:
		return singleBull
	}
	return math.Inf(1)
}

// BullingOff reports whether the players are still bulling off for who starts.
func (t *Turn) BullingOff() bool {
	return t.bull != nil && t.bull.contenders != nil
}

// BullOffDart is a dart thrown to bull off, as the game modes report it.
type BullOffDart struct {
	Entry    int   `json:"entry"` // index in the log
	PlayerID int64 `json:"player_id"`
	Round    int   `json:"round"` // 0 for everyone's first dart, then 1, 2, … for the ties
}

// tied reports whether the last round was a tie and nobody threw again yet.
func (b *bullOff) tied() bool {
	return b.contenders != nil && b.round > 0 && b.throws[len(b.throws)-1].Round != b.round
}
