package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// busyIn returns the game of a PlayerBusyError, and whether err is one.
func busyIn(err error) (int64, bool) {
	busy, ok := errors.AsType[PlayerBusyError](err)
	return busy.GameID, ok
}

// A player plays one running game at a time; lobbies, finished games and games they left
// don't count, and an account can still run games of other players at once.
func TestOneRunningGamePerPlayer(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, cara)
	befriend(t, s, ben, cara)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")

	// A lobby doesn't count.
	open := x01Game(anna.PlayerID)
	open.Seats = 2
	if _, err := s.CreateGame(ctx, anna, open, nil); err != nil {
		t.Fatal(err)
	}
	running, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil)
	if err != nil {
		t.Fatalf("a game while waiting in a lobby: %v", err)
	}
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID, papa.ID), nil); err == nil {
		t.Fatal("a second running game for anna")
	} else if id, ok := busyIn(err); !ok || id != running.Game.ID || !errors.Is(err, ErrInvalid) {
		t.Errorf("second game: %v, want PlayerBusyError for game %d", err, running.Game.ID)
	}
	// Other players of the account play at the same time, e.g. on another board.
	if _, err := s.CreateGame(ctx, anna, x01Game(papa.ID), nil); err != nil {
		t.Errorf("a guest's game next to anna's: %v", err)
	}

	// Taking a seat checks the joining player: invited or as a stranger.
	invited, err := s.CreateGame(ctx, ben, x01Game(ben.PlayerID), []string{"anna"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvitation(ctx, anna, invited.Game.ID, nil); err == nil {
		t.Error("anna accepts while she plays")
	} else if id, _ := busyIn(err); id != running.Game.ID {
		t.Errorf("accept: %v, want PlayerBusyError for game %d", err, running.Game.ID)
	}
	join := x01Game(cara.PlayerID)
	join.Seats = 2
	joinable, _ := s.CreateGame(ctx, cara, join, nil)
	if _, err := s.JoinGame(ctx, anna, joinable.Game.ID, nil); err == nil {
		t.Error("anna joins while she plays")
	}

	// A game the account takes no part in stays hidden.
	carasGame, err := s.CreateGame(ctx, cara, x01Game(cara.PlayerID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AcceptInvitation(ctx, ben, carasGame.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	err = s.checkPlayersFree(ctx, anna, []int64{cara.PlayerID})
	s.mu.Unlock()
	if id, ok := busyIn(err); !ok || id != 0 {
		t.Errorf("cara busy, seen by anna: %v (game %d), want PlayerBusyError without the game", err, id)
	}

	// A game that is over counts no more, though it can still be corrected; leaving one
	// (here: ending it) neither.
	quick := x01Game(anna.PlayerID)
	quick.Rules.(*x01.Settings).StartScore = 40
	if _, err := s.LeaveGame(ctx, anna, running.Game.ID); err != nil {
		t.Fatal(err)
	}
	over, err := s.CreateGame(ctx, anna, quick, nil)
	if err != nil {
		t.Fatalf("a game after leaving the last: %v", err)
	}
	if _, err := s.AddDart(ctx, anna, over.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	if v, err := s.JoinGame(ctx, anna, joinable.Game.ID, nil); err != nil || !v.Ready {
		t.Errorf("join after the last game is over: %v", err)
	}
}
