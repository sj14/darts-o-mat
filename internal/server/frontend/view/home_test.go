package view

import (
	"bytes"
	"context"
	"maps"
	"regexp"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// The start form preselects the settings it is given, and only those.
func TestNewGamePreselectsTheSettings(t *testing.T) {
	t.Parallel()
	checked := regexp.MustCompile(`name="([a-z_]+)" value="([a-z0-9_]*)" checked`)
	render := func(s game.Settings) map[string]string {
		t.Helper()
		var b bytes.Buffer
		if err := newGame(HomeModel{Mode: s.Mode, Settings: s}).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, m := range checked.FindAllStringSubmatch(b.String(), -1) {
			if _, twice := got[m[1]]; twice {
				t.Errorf("%s is checked twice", m[1])
			}
			got[m[1]] = m[2]
		}
		return got
	}
	x01Settings := game.Settings{Mode: game.ModeX01, Legs: 3, Sets: 2, BullOff: true, TurnSeconds: 120,
		Rules: &x01.Settings{StartScore: 301, OutMode: x01.OutModeMaster}}
	want := map[string]string{"start_score": "301", "out_mode": "master", "legs": "3", "sets": "2",
		"bull_off": "on", "turn_seconds": "120", "format": ""}
	if got := render(x01Settings); !maps.Equal(got, want) {
		t.Errorf("X01: checked %v, want %v", got, want)
	}

	atc := game.Settings{Mode: game.ModeAroundTheClock, Legs: 1, Sets: 1,
		Rules: &aroundtheclock.Settings{MultiplierSkips: true}}
	want = map[string]string{"multiplier_skips": "on", "legs": "1", "sets": "1", "turn_seconds": "0", "format": ""}
	if got := render(atc); !maps.Equal(got, want) {
		t.Errorf("around the clock: checked %v, want %v", got, want)
	}
}
