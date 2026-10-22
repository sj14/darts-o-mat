package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// quick is a game one dart ends: 40 with a double 20.
func quick(playerIDs ...int64) game.Game {
	g := x01Game(playerIDs...)
	g.Rules = &x01.Settings{StartScore: 40, OutMode: x01.OutModeSingle}
	return g
}

// A finished game can be corrected until a player goes on to the next game, also the one
// who lost it; then it can't be changed anymore.
func TestCompleteGame(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	v, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), []string{"ben"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	if _, err := s.AddDart(ctx, anna, id, d20); err != nil {
		t.Fatal(err)
	}

	// Until completed the last dart can be corrected, by the one who threw it.
	if v, _ = s.Game(ctx, anna, id); !v.Common().GameOver || v.Completed || !v.MayUndo() {
		t.Fatalf("after the last dart: over %v, completed %v, may undo %v; want over, open to corrections", v.Common().GameOver, v.Completed, v.MayUndo())
	}
	if _, err := s.CreateGame(ctx, ben, x01Game(ben.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); !v.Completed || v.Game.CompletedAt == nil {
		t.Fatalf("after ben, who lost, started another game: completed %v, stored %v", v.Completed, v.Game.CompletedAt)
	}
	// Afterwards nothing can be changed.
	if _, err := s.UndoDart(ctx, anna, id); !errors.Is(err, ErrCompleted) {
		t.Errorf("undo in a completed game: got %v, want ErrCompleted", err)
	}
	if _, err := s.EditDart(ctx, anna, id, v.Darts[0].ID, d20); !errors.Is(err, ErrCompleted) {
		t.Errorf("correct in a completed game: got %v, want ErrCompleted", err)
	}
	if v.MayUndo() || v.MayEdit(0) {
		t.Error("the view offers to change a completed game")
	}
}

// No timer runs for a game that finished a while ago: looking at it works out that its
// result is final, and stores that.
func TestCompletedByTime(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	v, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), nil)
	_, _ = s.AddDart(ctx, anna, v.Game.ID, d20)

	offset = game.CompleteAfter - 10*time.Second
	if v, _ = s.Game(ctx, anna, v.Game.ID); v.Completed {
		t.Error("completed before the minute is over")
	}
	offset = game.CompleteAfter + 2*time.Second // the database's time has whole seconds
	v, _ = s.Game(ctx, anna, v.Game.ID)
	if !v.Completed || v.Game.CompletedAt == nil {
		t.Errorf("a minute after the end: completed %v, stored %v; want completed and stored", v.Completed, v.Game.CompletedAt)
	}
	if left, _ := s.store.ListUnfinishedGamesByPlayer(ctx, anna.PlayerID); len(left) != 0 {
		t.Errorf("anna's unfinished games = %d, want none", len(left))
	}
	if _, err := s.UndoDart(ctx, anna, v.Game.ID); !errors.Is(err, ErrCompleted) {
		t.Errorf("undo after the minute: got %v, want ErrCompleted", err)
	}
}

// Starting another game completes the games that just finished, not those still running,
// so one account can keep several boards going.
func TestStartingAGameCompletesTheFinishedOnes(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	papa, _ := s.CreateGuest(ctx, anna, "Papa")

	running, _ := s.CreateGame(ctx, anna, x01Game(papa.ID), nil) // guests only: a second board
	_, _ = s.AddDart(ctx, anna, running.Game.ID, t20)
	finished, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), nil)
	_, _ = s.AddDart(ctx, anna, finished.Game.ID, d20)
	strangers, _ := s.CreateGame(ctx, ben, quick(ben.PlayerID), nil)
	_, _ = s.AddDart(ctx, ben, strangers.Game.ID, d20)

	if v, _ := s.Game(ctx, anna, finished.Game.ID); v.Completed {
		t.Fatal("completed before anything else happened")
	}
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Game(ctx, anna, finished.Game.ID); !v.Completed || v.Game.CompletedAt == nil {
		t.Error("the finished game isn't completed after anna started another")
	}
	if v, _ := s.Game(ctx, anna, running.Game.ID); v.Completed || v.Game.CompletedAt != nil || len(v.Darts) != 1 {
		t.Error("the running game was touched")
	}
	if v, _ := s.Game(ctx, ben, strangers.Game.ID); v.Completed {
		t.Error("another account's game was completed")
	}
}

// Accepting an invitation to the next game accepts the result of the last one, also for
// the one who lost it, so it is final before the next starts.
func TestAcceptingAGameCompletesTheFinishedOne(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, carla, ben)
	v, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), []string{"ben"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AddDart(ctx, anna, id, d20)

	next, err := s.CreateGame(ctx, carla, x01Game(carla.PlayerID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); v.Completed {
		t.Fatal("completed by an invitation ben hasn't answered")
	}
	if _, err := s.AcceptInvitation(ctx, ben, next.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); !v.Completed || v.Game.CompletedAt == nil {
		t.Error("the game isn't completed after ben, who lost it, accepted the next")
	}
}

// Whoever left a game has no say in its result.
func TestThoseWhoLeftCantComplete(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)
	v, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), []string{"ben", "carla"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AcceptInvitation(ctx, carla, id, nil)
	if _, err := s.LeaveGame(ctx, carla, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDart(ctx, anna, id, d20); err != nil {
		t.Fatal(err)
	}

	if v, _ = s.Game(ctx, carla, id); !v.HasLeft || !v.Common().GameOver {
		t.Fatalf("carla's view: left %v, over %v", v.HasLeft, v.Common().GameOver)
	}
	// Starting another game doesn't complete a game one has left.
	if _, err := s.CreateGame(ctx, carla, x01Game(carla.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); v.Completed {
		t.Error("a game was completed by someone who had left it")
	}
	// But whoever stayed accepts the result by starting another game, also without the
	// last dart, so it is final before the next one.
	if _, err := s.CreateGame(ctx, ben, x01Game(ben.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); !v.Completed || v.Game.CompletedAt == nil {
		t.Error("the game isn't completed after ben, who stayed, started another")
	}
}

// A game counts in the statistics once it is completed, and is stored then: it is not
// replayed again, and what is stored doesn't change anymore.
func TestStatsOfCompletedGames(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	anna := register(t, s, "anna")
	boardA, _ := s.CreateBoard(ctx, anna, "A")

	g := withBoard(quick(anna.PlayerID), boardA.ID)
	v, _ := s.CreateGame(ctx, anna, g, nil)
	x, y := 0.0, 100.0 // between the 20 and the inner side of the triple, at the top
	if _, err := s.AddBoardDart(ctx, boardA, throw.Dart{Number: 20, Segment: dartboard.Double, X: &x, Y: &y}); err != nil {
		t.Fatal(err)
	}

	got, err := s.Stats(ctx, anna, "")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].X01.Games != 0 {
		t.Errorf("a game that can still be corrected counts: %+v", got[0].X01)
	}
	if got[0].Heat.Darts != 1 {
		t.Errorf("heat = %+v, want the one camera dart", got[0].Heat)
	}
	if got, err = s.Stats(ctx, anna, game.ModeX01); err != nil || got[0].Heat.Darts != 1 {
		t.Errorf("heat of x01 = %+v, %v, want the one camera dart", got[0].Heat, err)
	}
	if got, err = s.Stats(ctx, anna, game.ModeAroundTheClock); err != nil || got[0].Heat.Darts != 0 {
		t.Errorf("heat of around the clock = %+v, %v, want no dart", got[0].Heat, err)
	}

	offset = game.CompleteAfter + time.Second
	for range 2 { // the second time it is read from the table
		if got, err = s.Stats(ctx, anna, ""); err != nil {
			t.Fatal(err)
		}
		if got[0].X01.Games != 1 || got[0].X01.Wins != 1 || got[0].X01.Darts != 1 {
			t.Fatalf("completed game: %+v, want it counted once", got[0].X01)
		}
	}
	if rs, _ := s.store.ListResults(ctx, []int64{anna.PlayerID}); len(rs) != 1 || rs[0].GameID != v.Game.ID {
		t.Errorf("stored results = %+v, want one for the game", rs)
	}
}
