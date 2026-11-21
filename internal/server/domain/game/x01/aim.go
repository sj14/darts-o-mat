package x01

import (
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// Aim returns the field a bot aims at with its next dart: the first dart of the checkout
// if the darts left in the turn can finish, else the treble 20, or near the finish a
// single that leaves a double.
func Aim(s Settings, st State, playerID int64) throw.Dart {
	p, _ := st.Player(playerID)
	darts := 3 - st.DartsInTurn
	// No dart scores more than 60, so a higher rest needs no search for a checkout.
	if p.Remaining <= 60*uint64(darts) {
		if route := s.OutMode.Checkout(p.Remaining, darts); route != nil {
			return route[0]
		}
	}
	return s.OutMode.setup(p.Remaining)
}

// setup is where to aim when the turn can't finish: the treble 20 while it can't bust,
// else a single that leaves a good double (finishOrder), else the highest single that
// doesn't bust.
func (m OutMode) setup(remaining uint64) throw.Dart {
	if remaining >= 60+m.minFinish() {
		return throw.Dart{Number: 20, Segment: dartboard.Triple}
	}
	for _, n := range finishOrder {
		leave := 2 * uint64(n)
		if n != dartboard.BullNumber && remaining > leave && remaining-leave <= 20 {
			return throw.Dart{Number: uint8(remaining - leave), Segment: dartboard.Single}
		}
	}
	if remaining >= 20+m.minFinish() {
		return throw.Dart{Number: 20, Segment: dartboard.Single}
	}
	return throw.Dart{Number: 1, Segment: dartboard.Single}
}
