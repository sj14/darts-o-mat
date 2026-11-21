package cricket

import (
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

func TestAim(t *testing.T) {
	t.Parallel()
	closed := [NumTargets]uint8{3, 3, 3, 3, 3, 3, 3}
	for _, tc := range []struct {
		name          string
		bot, opponent [NumTargets]uint8
		want          throw.Dart
	}{
		{"a new leg", [NumTargets]uint8{}, [NumTargets]uint8{}, throw.Dart{Number: 20, Segment: dartboard.Triple}},
		{"the 20 closed", [NumTargets]uint8{3}, [NumTargets]uint8{}, throw.Dart{Number: 19, Segment: dartboard.Triple}},
		{"only the bull open", [NumTargets]uint8{3, 3, 3, 3, 3, 3, 1}, [NumTargets]uint8{}, throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bullseye}},
		{"all closed, the opponent's 20 dead", closed, [NumTargets]uint8{3, 0}, throw.Dart{Number: 19, Segment: dartboard.Triple}},
	} {
		st := State{Players: []PlayerState{{PlayerID: 1, Marks: tc.bot}, {PlayerID: 2, Marks: tc.opponent}}}
		got := Aim(Settings{Variant: VariantStandard}, st, 1)
		if got.Number != tc.want.Number || got.Segment != tc.want.Segment {
			t.Errorf("%s: aims at %v, want %v", tc.name, got, tc.want)
		}
	}
}
