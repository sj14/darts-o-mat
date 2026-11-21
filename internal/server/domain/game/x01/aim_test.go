package x01

import (
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

func TestAim(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		out       OutMode
		remaining uint64
		thrown    int // darts of the turn thrown already
		want      throw.Dart
	}{
		{OutModeDouble, 501, 0, throw.Dart{Number: 20, Segment: dartboard.Triple}},
		{OutModeDouble, 170, 0, throw.Dart{Number: 20, Segment: dartboard.Triple}}, // the checkout begins with it
		{OutModeDouble, 40, 0, throw.Dart{Number: 20, Segment: dartboard.Double}},
		{OutModeDouble, 50, 2, throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bullseye}},
		{OutModeDouble, 41, 2, throw.Dart{Number: 1, Segment: dartboard.Single}},  // no checkout with one dart: leave D20
		{OutModeDouble, 75, 2, throw.Dart{Number: 20, Segment: dartboard.Triple}}, // can't bust on the treble
		{OutModeDouble, 61, 2, throw.Dart{Number: 20, Segment: dartboard.Single}}, // T20 would bust, no single leaves a double
		{OutModeSingle, 23, 2, throw.Dart{Number: 7, Segment: dartboard.Single}},  // leave 16
		{OutModeSingle, 20, 2, throw.Dart{Number: 20, Segment: dartboard.Single}}, // single out finishes on a single
		{OutModeMaster, 57, 2, throw.Dart{Number: 19, Segment: dartboard.Triple}}, // master out finishes on a treble
	} {
		st := State{State: turn.State{DartsInTurn: tc.thrown}, Players: []PlayerState{{PlayerID: 1, Remaining: tc.remaining}}}
		got := Aim(Settings{StartScore: 501, OutMode: tc.out}, st, 1)
		if got.Number != tc.want.Number || got.Segment != tc.want.Segment {
			t.Errorf("%v, %d left after %d darts: aims at %v, want %v", tc.out, tc.remaining, tc.thrown, got, tc.want)
		}
	}
}
