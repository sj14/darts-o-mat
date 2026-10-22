package store

import (
	"context"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// The settings of each mode are in a table of their own and come back with the game; a
// game that takes over the settings of another (a tournament's) shares the row.
func TestModeSettingsTables(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	create := func(s game.Settings) game.Game {
		g := game.Game{Settings: s, Participants: []game.Participant{{PlayerID: 1}, {PlayerID: 2}}}
		if err := st.CreateGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		return g
	}
	base := game.Settings{Legs: 1, Sets: 1}
	a := create(game.Settings{Mode: game.ModeX01, Legs: base.Legs, Sets: base.Sets, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}})
	b := create(game.Settings{Mode: game.ModeAroundTheClock, Legs: base.Legs, Sets: base.Sets, Rules: &aroundtheclock.Settings{Bull: true}})
	shared := create(a.Settings) // takes over a's row

	for _, tt := range []struct {
		id   int64
		want func(g game.Game) bool
	}{
		{a.ID, func(g game.Game) bool { r, ok := g.Rules.(*x01.Settings); return ok && r.StartScore == 501 }},
		{b.ID, func(g game.Game) bool { r, ok := g.Rules.(*aroundtheclock.Settings); return ok && r.Bull }},
		{shared.ID, func(g game.Game) bool {
			r, ok := g.Rules.(*x01.Settings)
			return ok && r.StartScore == 501 && g.SettingsID == a.SettingsID
		}},
	} {
		g, err := st.GetGame(ctx, tt.id)
		if err != nil {
			t.Fatal(err)
		}
		if !tt.want(g) {
			t.Errorf("game %d read back with settings %+v", tt.id, g.Rules)
		}
	}
}

// A game that is deleted takes its settings row along, unless another game still uses it.
func TestDeletingAGameDeletesItsSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	create := func(s game.Settings) game.Game {
		g := game.Game{Settings: s, Participants: []game.Participant{{PlayerID: 1, Status: game.InvitationAccepted}, {PlayerID: 2}}}
		if err := st.CreateGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		return g
	}
	rows := func() int64 {
		n, err := st.db.NewSelect().Model((*x01Row)(nil)).Count(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return int64(n)
	}
	a := create(game.Settings{Mode: game.ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}})
	b := create(a.Settings)
	// The one who accepted leaves: nobody is left, the game is deleted.
	for _, g := range []game.Game{a, b} {
		_ = st.RemoveParticipant(ctx, g.ID, 1)
		if g.ID == a.ID && rows() != 1 {
			t.Fatalf("the row went while a game still uses it: %d rows", rows())
		}
	}
	if rows() != 0 {
		t.Errorf("%d settings rows left, want none", rows())
	}
}
