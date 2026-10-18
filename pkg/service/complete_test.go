package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// quick is a game one dart ends: 40 with a double 20.
func quick(playerIDs ...int64) game.Game {
	g := x01Game(playerIDs...)
	g.Rules = &x01.Settings{StartScore: 40, OutMode: x01.OutModeSingle}
	return g
}

func TestCompleteGame(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	v, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), []string{"ben"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)

	if _, err := s.CompleteGame(ctx, anna, id); !errors.Is(err, ErrNotOver) {
		t.Errorf("complete a running game: got %v, want ErrNotOver", err)
	}
	if _, err := s.CompleteGame(ctx, carla, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("a stranger completes: got %v, want ErrNotFound", err)
	}
	if _, err := s.AddDart(ctx, anna, id, d20); err != nil {
		t.Fatal(err)
	}

	// Until confirmed the last dart can be corrected, by the one who threw it.
	if v, _ = s.Game(ctx, anna, id); !v.Common().GameOver || v.Completed || !v.MayUndo() {
		t.Fatalf("after the last dart: over %v, completed %v, may undo %v; want over, open to corrections", v.Common().GameOver, v.Completed, v.MayUndo())
	}
	// Whoever is in the game confirms, not only the winner.
	v, err := s.CompleteGame(ctx, ben, id)
	if err != nil || !v.Completed || v.Game.CompletedAt == nil {
		t.Fatalf("ben confirms: completed %v, %v", v.Completed, err)
	}
	if _, err := s.CompleteGame(ctx, anna, id); err != nil {
		t.Errorf("confirm twice: %v, want nothing to happen", err)
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

// Nothing is stored and no timer runs for a game that finished a while ago: looking at it
// works out that its result is final.
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
	if !v.Completed || v.Game.CompletedAt != nil {
		t.Errorf("a minute after the end: completed %v, stored %v; want completed by the time, nothing stored", v.Completed, v.Game.CompletedAt)
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
	if _, err := s.CompleteGame(ctx, carla, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("carla completes a game she left: got %v, want ErrForbidden", err)
	}
	// Starting another game isn't confirming the result of that one either.
	if _, err := s.CreateGame(ctx, carla, x01Game(carla.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); v.Completed {
		t.Error("a game was completed by someone who had left it")
	}
	if _, err := s.CompleteGame(ctx, ben, id); err != nil {
		t.Errorf("ben, who stayed, completes: %v", err)
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
	if _, err := s.AddBoardDart(ctx, boardA, throw.Dart{Number: 20, Segment: throw.Double, X: &x, Y: &y}); err != nil {
		t.Fatal(err)
	}

	got, err := s.Stats(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].X01.Games != 0 {
		t.Errorf("a game that can still be corrected counts: %+v", got[0].X01)
	}
	if got[0].Heat.Darts != 1 {
		t.Errorf("heat = %+v, want the one camera dart", got[0].Heat)
	}

	offset = game.CompleteAfter + time.Second
	for range 2 { // the second time it is read from the table
		if got, err = s.Stats(ctx, anna); err != nil {
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
