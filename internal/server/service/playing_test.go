package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

// busyIn returns the game and tournament of a PlayerBusyError, and whether err is one.
func busyIn(err error) (gameID, tournamentID int64, ok bool) {
	busy, ok := errors.AsType[PlayerBusyError](err)
	return busy.GameID, busy.TournamentID, ok
}

// A player has one seat at a time: in a lobby, a running game or a tournament that isn't
// over. Invitations, finished games and games they left don't count, and an account can
// still run games of other players at once.
func TestOneSeatPerPlayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, anna, ben)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")

	// A lobby counts, until anna leaves it.
	open := x01Game(anna.PlayerID)
	open.Seats = 2
	open.Settings.TurnSeconds = 60
	lobby, err := createGame(ctx, s, anna, open, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil); err == nil {
		t.Fatal("a game while waiting in a lobby")
	} else if g, _, ok := busyIn(err); !ok || g != lobby.Game.ID || !errors.Is(err, ErrInvalid) {
		t.Errorf("game while in a lobby: %v, want PlayerBusyError for game %d", err, lobby.Game.ID)
	}
	if _, err := s.LeaveGame(ctx, anna, lobby.Game.ID); err != nil {
		t.Fatal(err)
	}

	running, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil)
	if err != nil {
		t.Fatalf("a game after leaving the lobby: %v", err)
	}
	if _, err := createGame(ctx, s, anna, x01Game(anna.PlayerID, papa.ID), nil); err == nil {
		t.Fatal("a second running game for anna")
	} else if g, _, _ := busyIn(err); g != running.Game.ID {
		t.Errorf("second game: %v, want PlayerBusyError for game %d", err, running.Game.ID)
	}
	// Other players of the account play at the same time, e.g. on another board.
	if _, err := createGame(ctx, s, anna, x01Game(papa.ID), nil); err != nil {
		t.Errorf("a guest's game next to anna's: %v", err)
	}

	// An invitation doesn't count; taking the seat checks the player, invited or as a
	// stranger, in a game or a tournament.
	invited, err := createGame(ctx, s, ben, x01Game(ben.PlayerID), []string{"anna"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acceptGame(ctx, s, anna, invited.Game.ID, nil); err == nil {
		t.Error("anna accepts while she plays")
	} else if g, _, _ := busyIn(err); g != running.Game.ID {
		t.Errorf("accept: %v, want PlayerBusyError for game %d", err, running.Game.ID)
	}
	joinable := quickTournament(tournament.RoundRobin, cara.PlayerID)
	joinable.Seats = 3
	joinable.Settings.TurnSeconds = 60
	tv, err := createTournament(ctx, s, cara, joinable, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := joinTournament(ctx, s, anna, tv.Tournament.ID, nil); err == nil {
		t.Error("anna joins a tournament while she plays")
	}

	// A seat the account takes no part in stays hidden: ben waits in his own lobby.
	unlock, err := s.lock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = s.checkPlayersFree(ctx, cara, []int64{ben.PlayerID}, 0)
	unlock()
	if g, tr, ok := busyIn(err); !ok || g != 0 || tr != 0 {
		t.Errorf("ben busy, seen by cara: %v, want PlayerBusyError without a link", err)
	}

	// A game that is over counts no more, though it can still be corrected; a game left
	// (here: ended) neither.
	if _, err := s.LeaveGame(ctx, anna, running.Game.ID); err != nil {
		t.Fatal(err)
	}
	over, err := createGame(ctx, s, anna, quick(anna.PlayerID), nil)
	if err != nil {
		t.Fatalf("a game after leaving the last: %v", err)
	}
	if _, err := s.AddDart(ctx, anna, over.Game.ID, d20); err != nil {
		t.Fatal(err)
	}

	// A tournament counts as a whole, from its lobby on and also between its matches.
	if _, err := joinTournament(ctx, s, anna, tv.Tournament.ID, nil); err != nil {
		t.Fatalf("join after the last game is over: %v", err)
	}
	if _, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil); err == nil {
		t.Error("a game while in a tournament")
	} else if _, tr, _ := busyIn(err); tr != tv.Tournament.ID {
		t.Errorf("game while in a tournament: %v, want PlayerBusyError for tournament %d", err, tv.Tournament.ID)
	}
	if _, err := createTournament(ctx, s, anna, quickTournament(tournament.RoundRobin, anna.PlayerID), nil); err == nil {
		t.Error("a second tournament for anna")
	}
}

// A lobby stays offered as long as someone is in it, and is gone once the last one left.
func TestLobbyGoneWithTheLastPlayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")

	tr := quickTournament(tournament.RoundRobin, anna.PlayerID)
	tr.Seats = 3
	tr.Settings.TurnSeconds = 60
	tv, err := createTournament(ctx, s, anna, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := joinTournament(ctx, s, ben, tv.Tournament.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LeaveTournament(ctx, anna, tv.Tournament.ID); err != nil {
		t.Fatal(err)
	}
	if l, _ := s.OpenLobbies(ctx, cara); len(l) != 1 || len(l[0].Joined) != 1 {
		t.Fatalf("open lobbies after anna left = %+v, want the tournament with ben", l)
	}
	if _, err := s.LeaveTournament(ctx, ben, tv.Tournament.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.GetTournament(ctx, tv.Tournament.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the tournament everybody left: %v, want it deleted", err)
	}
	if l, _ := s.OpenLobbies(ctx, cara); len(l) != 0 {
		t.Errorf("open lobbies after everybody left = %+v, want none", l)
	}
}
