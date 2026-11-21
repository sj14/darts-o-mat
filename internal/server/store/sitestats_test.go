package store

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/site"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
)

// SiteStats counts the rows of each kind, the recent ones by their time, on both databases.
func TestSiteStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	testSiteStats(t, st)

	if os.Getenv(storetest.PostgresEnv) != "" {
		testSiteStats(t, newPostgresDatabase(t, "darts_o_mat_site_stats"))
	}
}

func testSiteStats(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	lastMonth := now.AddDate(0, 0, -site.RecentDays-1)
	insert := func(model any) {
		t.Helper()
		if _, err := st.db.NewInsert().Model(model).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}

	anna, ben := createAccount(t, st, "anna"), createAccount(t, st, "ben")
	createAccount(t, st, "carl") // registered now, never active
	dora := createAccount(t, st, "dora")
	if _, err := st.db.NewUpdate().Model((*account.Account)(nil)).Set("last_seen_at = ?", yesterday).Where("id = ?", anna.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.NewUpdate().Model((*account.Account)(nil)).Set("last_seen_at = ?", now.AddDate(0, 0, -10)).Set("created_at = ?", lastMonth).Where("id = ?", ben.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDisabled(ctx, dora.ID, true); err != nil {
		t.Fatal(err)
	}
	insert(&player.Player{Name: "guest", GuestOf: &anna.ID})
	insert(&player.Player{Name: "deleted guest", GuestOf: &anna.ID, DeletedAt: &yesterday})
	bot := player.Player{Name: "Bot", GuestOf: &anna.ID, Bot: true} // neither a guest nor its darts count
	insert(&bot)

	x01 := game.Settings{Mode: game.ModeX01}
	insert(&game.Game{Settings: x01})                                                 // in the lobby
	insert(&game.Game{Settings: x01, StartedAt: &yesterday})                          // running
	insert(&game.Game{Settings: x01, StartedAt: &yesterday, CompletedAt: &yesterday}) // completed lately
	insert(&game.Game{Settings: x01, StartedAt: &lastMonth, CompletedAt: &lastMonth})
	insert(&game.Game{Settings: game.Settings{Mode: game.ModeCricket}, StartedAt: &yesterday, CompletedAt: &yesterday})
	insert(&game.Game{Settings: x01, StartedAt: &yesterday, AbortedAt: &yesterday})

	insert(&tournament.Tournament{Format: tournament.Knockout, Settings: x01}) // in the lobby
	insert(&tournament.Tournament{Format: tournament.Knockout, Settings: x01, StartedAt: &yesterday})
	insert(&tournament.Tournament{Format: tournament.Knockout, Settings: x01, StartedAt: &lastMonth, CompletedAt: &yesterday})

	x, y := 1.0, 2.0
	insert(&throw.Dart{GameID: 2, PlayerID: anna.PlayerID, Number: 20, Segment: 1, CreatedAt: yesterday})
	insert(&throw.Dart{GameID: 2, PlayerID: anna.PlayerID, Number: 20, Segment: 1, X: &x, Y: &y, CreatedAt: yesterday})
	insert(&throw.Dart{GameID: 2, PlayerID: anna.PlayerID, Takeout: true, CreatedAt: yesterday})
	insert(&throw.Dart{GameID: 2, PlayerID: bot.ID, Number: 20, Segment: 1, X: &x, Y: &y, CreatedAt: yesterday})
	insert(&throw.Dart{GameID: 4, PlayerID: anna.PlayerID, Number: 20, Segment: 1, X: &x, Y: &y, CreatedAt: lastMonth}) // a game started before

	boards := []board.Board{{AccountID: anna.ID, Name: "a", LastUsedAt: &yesterday}, {AccountID: anna.ID, Name: "b", LastUsedAt: &lastMonth}, {AccountID: ben.ID, Name: "c"}}
	for i := range boards {
		if err := st.CreateBoard(ctx, &boards[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetBoardToken(ctx, boards[1].ID, "hash"); err != nil {
		t.Fatal(err)
	}

	got, err := st.SiteStats(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	want := site.Stats{
		Accounts:    site.AccountStats{Total: 4, Active: 1, ActiveRecent: 2, NewRecent: 3, Disabled: 1},
		Guests:      1,
		Games:       site.GameStats{Running: 1, Completed: 3, Aborted: 1, CompletedRecent: 2, CompletedByMode: map[game.Mode]int{game.ModeX01: 2, game.ModeCricket: 1}},
		Tournaments: site.TournamentStats{Running: 1, Completed: 1},
		Darts:       site.DartStats{Recent: 2, Camera: 1},
		Boards:      site.BoardStats{Total: 3, Camera: 1, UsedRecent: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
