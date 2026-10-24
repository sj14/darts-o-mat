package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
)

// withForm stores three X01 results of the account's player with the average.
func withForm(t *testing.T, s *Service, acc account.Account, average uint64) {
	t.Helper()
	var results []stats.Result
	for i := range int64(stats.MinFormGames) {
		results = append(results, stats.Result{GameID: acc.PlayerID*100 + i, PlayerID: acc.PlayerID, Mode: game.ModeX01, Darts: 3, Points: average})
	}
	if err := s.store.AddResults(context.Background(), results); err != nil {
		t.Fatal(err)
	}
}

// Without a form both players fit anyone: the first waits in a new quick match with its
// rules, the second joins it and it starts.
func TestQuickMatch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	if _, err := s.QuickMatch(ctx, anna, "darts", nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown mode: %v, want ErrInvalid", err)
	}
	waiting, err := s.QuickMatch(ctx, anna, game.ModeX01, nil)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Started || !IsQuickMatch(waiting.Game) || waiting.Game.TurnSeconds != 60 || !waiting.Game.Open() {
		t.Fatalf("anna's quick match: started %v, quick %v, %+v; want an open one waiting with a minute per turn", waiting.Started, IsQuickMatch(waiting.Game), waiting.Game.Settings)
	}
	if moved, next, err := s.QuickMatchMove(ctx, anna, waiting.Game.ID); err != nil || moved != waiting.Game.ID || !next.IsZero() {
		t.Errorf("anna, alone, moves to %d, next check %v, %v; want to stay, no check", moved, next, err)
	}
	v, err := s.QuickMatch(ctx, ben, game.ModeX01, nil)
	if err != nil || v.Game.ID != waiting.Game.ID || !v.Started {
		t.Fatalf("ben's quick match: game %d started %v, %v; want anna's, started", v.Game.ID, v.Started, err)
	}
	// Around the clock is another queue.
	carla := register(t, s, "carla")
	if v, err := s.QuickMatch(ctx, carla, game.ModeAroundTheClock, nil); err != nil || v.Started {
		t.Errorf("carla's around the clock: started %v, %v; want waiting", v.Started, err)
	}
}

// Players of different strength wait apart at first; as the older quick match waits, the
// difference it allows grows, until the newer one moves into it.
func TestQuickMatchWidens(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	withForm(t, s, anna, 40)
	withForm(t, s, ben, 70)
	withForm(t, s, carla, 44)

	annas, err := s.QuickMatch(ctx, anna, game.ModeX01, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The filter: carla is close to anna, ben isn't yet.
	if l, _ := s.OpenLobbies(ctx, carla); len(l) != 1 || !l[0].Fits {
		t.Errorf("carla's open games = %+v, want anna's fitting", l)
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 1 || l[0].Fits || len((LobbyFilter{Fits: true}).Apply(l)) != 0 {
		t.Errorf("ben's open games = %+v, want anna's not fitting", l)
	}

	bens, err := s.QuickMatch(ctx, ben, game.ModeX01, nil)
	if err != nil || bens.Game.ID == annas.Game.ID || bens.Started {
		t.Fatalf("ben's quick match: game %d started %v, %v; want one of his own, waiting", bens.Game.ID, bens.Started, err)
	}
	if moved, next, _ := s.QuickMatchMove(ctx, ben, bens.Game.ID); moved != bens.Game.ID || !next.Equal(annas.Game.CreatedAt.Add(150*time.Second)) {
		t.Errorf("ben: moved to %d, next check %v; want to stay until anna's game waited 2m30s (%v)", moved, next, annas.Game.CreatedAt.Add(150*time.Second))
	}

	offset = 150 * time.Second // 6 steps of 5: a difference of 30 fits
	if moved, next, err := s.QuickMatchMove(ctx, anna, annas.Game.ID); err != nil || moved != annas.Game.ID || !next.IsZero() {
		t.Errorf("anna, the older, moves to %d, next check %v, %v; want to stay, no check", moved, next, err)
	}
	moved, _, err := s.QuickMatchMove(ctx, ben, bens.Game.ID)
	if err != nil || moved != annas.Game.ID {
		t.Fatalf("ben moves to %d, %v; want anna's game", moved, err)
	}
	if v, err := s.Game(ctx, ben, moved); err != nil || !v.Started {
		t.Errorf("anna's game after ben came: started %v, %v", v.Started, err)
	}
	if _, err := s.Game(ctx, ben, bens.Game.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben's own quick match after moving: %v, want it gone", err)
	}
}
