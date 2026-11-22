package turn

import (
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// bullOff decides who starts the first leg: each contender throws one dart at the
// bull, the closest starts. Equally close players throw again, unless the players
// decided who was closer. Who takes too long for their dart or leaves the game is out.
type bullOff struct {
	contenders []int       // players still bulling off, in throwing order; nil once decided
	closer     *int        // see Log.BullOffCloser
	round      int         // 0 for everyone's first dart, then the ties
	darts      []roundDart // the darts of this round of the players still in it
	throws     []bullOffThrow
}

// roundDart is the dart a player threw in the round being bulled off.
type roundDart struct {
	player int
	dart   throw.Dart
}

// bullOffThrow is a dart thrown to bull off.
type bullOffThrow struct {
	Entry  int // index in the log
	Player int
	Round  int // 0 for everyone's first dart, then 1, 2, … for the ties
}

func newBullOff(players int, closer *int) *bullOff {
	b := &bullOff{closer: closer}
	for i := range players {
		b.contenders = append(b.contenders, i)
	}
	return b
}

// throw counts the bull-off dart of player. It returns who throws next, or the
// starter of the first leg once that is decided.
func (b *bullOff) throw(entry, player int, d throw.Dart) (next int, decided bool) {
	b.throws = append(b.throws, bullOffThrow{Entry: entry, Player: player, Round: b.round})
	b.darts = append(b.darts, roundDart{player: player, dart: d})
	if i := slices.Index(b.contenders, player); i+1 < len(b.contenders) {
		return b.contenders[i+1], false
	}
	return b.endRound()
}

// out takes player out of bulling off, who took too long for their dart (Turn.Skip) or
// left the game (Turn.Leave): they can't be the closest anymore and don't throw again
// after a tie. up is the player who throws now. It returns who throws next, or the
// starter of the first leg once that is decided: the only one left starts at once.
func (b *bullOff) out(player, up int) (next int, decided bool) {
	b.darts = slices.DeleteFunc(b.darts, func(d roundDart) bool { return d.player == player })
	i := slices.Index(b.contenders, player)
	if i < 0 { // out already after a tie
		return up, false
	}
	b.contenders = slices.Delete(b.contenders, i, i+1)
	switch {
	case len(b.contenders) == 1:
		starter := b.contenders[0]
		b.contenders = nil
		return starter, true
	case player != up:
		return up, false
	case i < len(b.contenders):
		return b.contenders[i], false
	}
	return b.endRound()
}

// endRound decides once everyone in the round threw or is out (someone threw, else only
// one would be left, see out): the closest starts, those equally close throw again.
func (b *bullOff) endRound() (next int, decided bool) {
	closest := closestOf(b.darts)
	if b.closer != nil && slices.Contains(closest, *b.closer) {
		closest = []int{*b.closer}
	}
	if len(closest) == 1 {
		b.contenders = nil
		return closest[0], true
	}
	b.contenders, b.darts = closest, nil
	b.round++
	return b.contenders[0], false
}

// singleBull is where a single bull without a position counts, in mm from the center.
const singleBull = 11

// closestOf returns the players whose darts of a round came closest to the bull. The
// positions count only if every dart of the round has one (cameras, bots); else all count
// by their field, so a dart typed in by hand is never farther than one with a position
// next to it: the bullseye, then the bull, the rest equal.
func closestOf(darts []roundDart) []int {
	positions := !slices.ContainsFunc(darts, func(d roundDart) bool { return d.dart.X == nil || d.dart.Y == nil })
	best := math.Inf(1)
	var closest []int
	for _, d := range darts {
		switch dist := bullDistance(d.dart, positions); {
		case dist < best:
			best, closest = dist, []int{d.player}
		case dist == best:
			closest = append(closest, d.player)
		}
	}
	return closest
}

// bullDistance is how far from the center of the bull a dart hit, in mm: from the
// position the cameras saw if positions count; else 0 for the bullseye, the middle of
// the bull for a single bull, infinite otherwise.
func bullDistance(d throw.Dart, positions bool) float64 {
	switch {
	case positions:
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
