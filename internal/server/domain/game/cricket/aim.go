package cricket

import (
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// Aim returns the field a bot aims at with its next dart: the treble of the highest
// number it hasn't closed, once all are closed the highest number that still scores.
func Aim(_ Settings, st State, playerID int64) throw.Dart {
	p, _ := st.Player(playerID)
	for k, n := range Targets {
		if p.Marks[k] < MarksToClose {
			return best(n)
		}
	}
	for k, n := range Targets {
		if !st.Dead(k) {
			return best(n)
		}
	}
	return best(Targets[0])
}

// best is the field of a number that scores the most marks: the treble, or the bullseye.
func best(number uint8) throw.Dart {
	if number == dartboard.BullNumber {
		return throw.Dart{Number: number, Segment: dartboard.Bullseye}
	}
	return throw.Dart{Number: number, Segment: dartboard.Triple}
}
