package aroundtheclock

import (
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// Aim returns the field a bot aims at with its next dart: the single of its target, the
// largest field of the number, or the bull at the end.
func Aim(_ Settings, st State, playerID int64) throw.Dart {
	p, _ := st.Player(playerID)
	if p.Target == dartboard.BullNumber {
		return throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bull}
	}
	return throw.Dart{Number: p.Target, Segment: dartboard.Single}
}
