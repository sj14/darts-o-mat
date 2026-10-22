package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// A game begins as a lobby with the player of the account who created it; players are
// added and friends invited there, and a player starts it, which draws the order.
func TestGameLobby(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, anna, ben)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	// The draw reverses the order the players took their seats.
	s.shuffle = func(n int, swap func(i, j int)) {
		for i := range n / 2 {
			swap(i, n-1-i)
		}
	}

	v, err := s.CreateGame(ctx, anna, x01Game(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, lobby := v.Game.ID, GameLobby(v.Game.ID)
	if v.Started || !v.CanStart || !slices.Equal(v.Game.PlayerIDs(), []int64{anna.PlayerID}) {
		t.Fatalf("new game: started %v, can start %v, players %v; want a lobby with anna", v.Started, v.CanStart, v.Game.PlayerIDs())
	}
	if v.Choices == nil || len(v.Choices.Players) != 1 || len(v.Choices.Friends) != 1 {
		t.Errorf("choices = %+v, want Papa to add and ben to invite", v.Choices)
	}
	if _, err := s.AddDart(ctx, anna, id, t20); !errors.Is(err, ErrNotReady) {
		t.Errorf("dart in the lobby: got %v, want ErrNotReady", err)
	}

	if err := s.AddPlayer(ctx, anna, lobby, papa.ID, &keller.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AddPlayer(ctx, anna, lobby, papa.ID, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("adding twice: got %v, want ErrInvalid", err)
	}
	if err := s.SetSeatBoard(ctx, anna, lobby, anna.PlayerID, &keller.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Invite(ctx, anna, lobby, "ben"); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx, anna, lobby); !errors.Is(err, ErrNotReady) {
		t.Errorf("start with ben invited: got %v, want ErrNotReady", err)
	}
	for _, err := range []error{s.Start(ctx, cara, lobby), s.AddPlayer(ctx, cara, lobby, cara.PlayerID, nil), s.Invite(ctx, cara, lobby, "anna")} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("an outsider in the lobby: got %v, want ErrNotFound", err)
		}
	}
	if err := s.Start(ctx, ben, lobby); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben starts before accepting: got %v, want ErrForbidden", err)
	}
	if err := s.Accept(ctx, ben, lobby, nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); v.Started || !v.CanStart {
		t.Fatalf("after ben accepted: started %v, can start %v; want it waiting to be started", v.Started, v.CanStart)
	}

	// Anyone in it may start it.
	if err := s.Start(ctx, ben, lobby); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Game(ctx, anna, id)
	if want := []int64{ben.PlayerID, papa.ID, anna.PlayerID}; !v.Started || !slices.Equal(v.Game.PlayerIDs(), want) {
		t.Errorf("started %v in the order %v, want %v as drawn", v.Started, v.Game.PlayerIDs(), want)
	}
	if !onBoard(v, anna.PlayerID, keller.ID) || !onBoard(v, papa.ID, keller.ID) {
		t.Error("anna and Papa don't throw on Keller")
	}
	if err := s.AddPlayer(ctx, anna, lobby, papa.ID, nil); !errors.Is(err, ErrStarted) {
		t.Errorf("adding after the start: got %v, want ErrStarted", err)
	}
	if _, err := s.AddDart(ctx, ben, id, t20); err != nil {
		t.Errorf("first dart, ben's as drawn: %v", err)
	}
}

// Taking players out of the lobby: the last one ends it; a guest's game can begin while the
// account's own player plays elsewhere.
func TestRemoveFromLobby(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")

	v, _ := s.CreateGame(ctx, anna, x01Game(), 0, nil)
	lobby := GameLobby(v.Game.ID)
	_ = s.AddPlayer(ctx, anna, lobby, papa.ID, nil)
	_ = s.Invite(ctx, anna, lobby, "ben")
	if err := s.RemovePlayer(ctx, anna, lobby, ben.PlayerID); !errors.Is(err, ErrNotFound) {
		t.Errorf("anna takes out ben: got %v, want ErrNotFound", err)
	}
	if err := s.RemovePlayer(ctx, anna, lobby, anna.PlayerID); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, v.Game.ID); !slices.Equal(v.Game.PlayerIDs(), []int64{papa.ID, ben.PlayerID}) {
		t.Errorf("players = %v, want Papa and ben invited", v.Game.PlayerIDs())
	}
	if err := s.RemovePlayer(ctx, anna, lobby, papa.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Game(ctx, ben, v.Game.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the lobby nobody is in anymore: %v, want it deleted with ben's invitation", err)
	}

	// Anna plays; Papa's game begins without her.
	if _, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGame(ctx, anna, x01Game(), 0, nil); err == nil {
		t.Error("a lobby with anna while she plays")
	}
	if v, err := s.CreateGame(ctx, anna, x01Game(), papa.ID, nil); err != nil || !slices.Equal(v.Game.PlayerIDs(), []int64{papa.ID}) {
		t.Errorf("Papa's game: %v, %v; want it with Papa", v.Game.PlayerIDs(), err)
	}
}

// A tournament starts like a game, once it has enough players; the draw is made then.
func TestTournamentLobby(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	g1, _ := s.CreateGuest(ctx, anna, "Gast 1")
	g2, _ := s.CreateGuest(ctx, anna, "Gast 2")

	v, err := s.CreateTournament(ctx, anna, quickTournament(tournament.RoundRobin), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	lobby := TournamentLobby(v.Tournament.ID)
	_ = s.AddPlayer(ctx, anna, lobby, g1.ID, nil)
	if err := s.Start(ctx, anna, lobby); !errors.Is(err, tournament.ErrPlayerCount) {
		t.Errorf("start with two players: got %v, want ErrPlayerCount", err)
	}
	_ = s.AddPlayer(ctx, anna, lobby, g2.ID, nil)
	if err := s.Start(ctx, anna, lobby); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Tournament(ctx, anna, v.Tournament.ID); !v.Started || len(v.State.Matches) != 3 {
		t.Errorf("started %v with %d matches, want 3 of all against all", v.Started, len(v.State.Matches))
	}
}

// An open lobby starts by itself when its last seat is taken.
func TestOpenLobbyStartsWhenFull(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	open := x01Game()
	open.Seats = 2
	v, _ := s.CreateGame(ctx, anna, open, 0, nil)
	if err := s.Start(ctx, anna, GameLobby(v.Game.ID)); !errors.Is(err, ErrNotReady) {
		t.Errorf("start with a seat free: got %v, want ErrNotReady", err)
	}
	if err := s.Join(ctx, ben, GameLobby(v.Game.ID), nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, v.Game.ID); !v.Started {
		t.Error("the open game didn't start with its last seat")
	}
}
