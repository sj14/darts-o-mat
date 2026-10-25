package store

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
)

// ListRecentResults gives each player's most recent results of the mode, at most the limit,
// on both databases (a window function).
func TestListRecentResults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	testListRecentResults(t, st)

	if os.Getenv(storetest.PostgresEnv) != "" {
		testListRecentResults(t, newPostgresDatabase(t, "darts_o_mat_recent_results"))
	}
}

func testListRecentResults(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	var results []stats.Result
	for id := int64(1); id <= 25; id++ { // player 1: 25 games of X01
		results = append(results, stats.Result{GameID: id, PlayerID: 1, Mode: game.ModeX01})
	}
	results = append(results,
		stats.Result{GameID: 26, PlayerID: 1, Mode: game.ModeAroundTheClock},
		stats.Result{GameID: 1, PlayerID: 2, Mode: game.ModeX01},
		stats.Result{GameID: 2, PlayerID: 2, Mode: game.ModeX01},
		stats.Result{GameID: 3, PlayerID: 3, Mode: game.ModeX01}, // not asked for
	)
	if err := st.AddResults(ctx, results); err != nil {
		t.Fatal(err)
	}

	got, err := st.ListRecentResults(ctx, []int64{1, 2}, game.ModeX01, 20)
	if err != nil {
		t.Fatal(err)
	}
	games := map[int64][]int64{}
	for _, r := range got {
		if r.Mode != game.ModeX01 {
			t.Errorf("a result of %s", r.Mode)
		}
		games[r.PlayerID] = append(games[r.PlayerID], r.GameID)
	}
	for _, ids := range games {
		slices.Sort(ids)
	}
	if want := []int64{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25}; !slices.Equal(games[1], want) {
		t.Errorf("player 1: games %v, want the 20 most recent", games[1])
	}
	if !slices.Equal(games[2], []int64{1, 2}) || len(games) != 2 {
		t.Errorf("games = %v, want player 2's two and nobody else's", games)
	}
}
