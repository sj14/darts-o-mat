package turn

import (
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// BullOffThrow is a dart thrown to bull off.
type BullOffThrow struct {
	Entry  int // index in the log
	Player int
	// Distance from the center of the bull in mm, from the position the cameras saw.
	// Without a position: 0 for the bullseye, the middle of the bull for a single bull,
	// infinite otherwise.
	Distance float64
	Round    int // 0 for everyone's first dart, then 1, 2, … for the ties
}

// singleBull is where a single bull without a position counts, in mm from the center.
const singleBull = 11

func bullDistance(d throw.Dart) float64 {
	switch {
	case d.X != nil && d.Y != nil:
		return math.Hypot(*d.X, *d.Y)
	case d.Number == throw.Bull && d.Multiplier == 2:
		return 0
	case d.Number == throw.Bull:
		return singleBull
	}
	return math.Inf(1)
}

// BullOffDart is a dart thrown to bull off, as the game modes report it.
type BullOffDart struct {
	Entry    int   `json:"entry"` // index in the log
	PlayerID int64 `json:"player_id"`
	Round    int   `json:"round"` // 0 for everyone's first dart, then 1, 2, … for the ties
}

// BullOffDarts returns the bull-off darts with the ids of the players.
func (t *Turn) BullOffDarts(playerIDs []int64) []BullOffDart {
	darts := []BullOffDart{}
	for _, b := range t.BullOff {
		darts = append(darts, BullOffDart{Entry: b.Entry, PlayerID: playerIDs[b.Player], Round: b.Round})
	}
	return darts
}

// ThrowerIDs returns the id of the player who threw each log entry, 0 for takeouts.
func (t *Turn) ThrowerIDs(playerIDs []int64) []int64 {
	ids := make([]int64, len(t.Throwers))
	for i, p := range t.Throwers {
		if p >= 0 {
			ids[i] = playerIDs[p]
		}
	}
	return ids
}

// BullingOff reports whether the players are still bulling off for who starts.
func (t *Turn) BullingOff() bool {
	return len(t.contenders) > 0
}

// BullOffTie returns the players who are equally close and throw again, while no one
// threw again yet; nil otherwise.
func (t *Turn) BullOffTie(playerIDs []int64) []int64 {
	if t.bullOffRound == 0 || len(t.BullOff) == 0 || t.BullOff[len(t.BullOff)-1].Round == t.bullOffRound {
		return nil
	}
	var ids []int64
	for _, p := range t.contenders {
		ids = append(ids, playerIDs[p])
	}
	return ids
}

// bullOff counts a dart thrown to bull off. Once every contender threw, the closest
// starts the first leg; if several are equally close, they throw again, unless the
// players decided who was closer (Log.BullOffCloser).
func (t *Turn) bullOff(entry int, d throw.Dart) {
	t.BullOff = append(t.BullOff, BullOffThrow{Entry: entry, Player: t.Player, Distance: bullDistance(d), Round: t.bullOffRound})
	if i := slices.Index(t.contenders, t.Player); i+1 < len(t.contenders) {
		t.Player = t.contenders[i+1]
		return
	}

	best := math.Inf(1)
	var closest []int
	for _, b := range t.BullOff {
		switch {
		case b.Round != t.bullOffRound:
		case b.Distance < best:
			best, closest = b.Distance, []int{b.Player}
		case b.Distance == best:
			closest = append(closest, b.Player)
		}
	}
	if t.closer != nil && slices.Contains(closest, *t.closer) {
		closest = []int{*t.closer}
	}
	if len(closest) == 1 {
		t.first, t.Player, t.contenders = closest[0], closest[0], nil
		return
	}
	t.contenders, t.bullOffRound, t.Player = closest, t.bullOffRound+1, closest[0]
}
