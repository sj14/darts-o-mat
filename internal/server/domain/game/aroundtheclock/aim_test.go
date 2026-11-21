package aroundtheclock

import (
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

func TestAim(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		target uint8
		want   throw.Dart
	}{
		{1, throw.Dart{Number: 1, Segment: dartboard.Single}},
		{20, throw.Dart{Number: 20, Segment: dartboard.Single}},
		{dartboard.BullNumber, throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bull}},
	} {
		st := State{Players: []PlayerState{{PlayerID: 1, Target: tc.target}}}
		got := Aim(Settings{Bull: true}, st, 1)
		if got.Number != tc.want.Number || got.Segment != tc.want.Segment {
			t.Errorf("target %d: aims at %v, want %v", tc.target, got, tc.want)
		}
	}
}
