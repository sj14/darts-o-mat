package store

import (
	"context"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
)

// The settings of each mode are in a table of their own and come back with the game; a
// game that takes over the settings of another (a tournament's) shares the row.
func TestModeSettingsTables(t *testing.T) {
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	create := func(s game.Settings) game.Game {
		g := game.Game{Settings: s, Participants: []game.Participant{{PlayerID: 1}, {PlayerID: 2}}}
		if err := st.CreateGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		return g
	}
	base := game.Settings{Legs: 1, Sets: 1}
	a := create(game.Settings{Mode: game.ModeX01, Legs: base.Legs, Sets: base.Sets, X01: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}})
	b := create(game.Settings{Mode: game.ModeAroundTheClock, Legs: base.Legs, Sets: base.Sets, AroundTheClock: &aroundtheclock.Settings{Bull: true}})
	shared := create(a.Settings) // takes over a's row

	for _, tt := range []struct {
		id   int64
		want func(g game.Game) bool
	}{
		{a.ID, func(g game.Game) bool { return g.X01 != nil && g.X01.StartScore == 501 && g.AroundTheClock == nil }},
		{b.ID, func(g game.Game) bool { return g.AroundTheClock != nil && g.AroundTheClock.Bull && g.X01 == nil }},
		{shared.ID, func(g game.Game) bool { return g.X01 != nil && g.X01.StartScore == 501 && g.SettingsID == a.SettingsID }},
	} {
		g, err := st.GetGame(ctx, tt.id)
		if err != nil {
			t.Fatal(err)
		}
		if !tt.want(g) {
			t.Errorf("game %d read back with settings %+v, %+v", tt.id, g.X01, g.AroundTheClock)
		}
	}
}
