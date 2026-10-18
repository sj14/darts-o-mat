package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
)

func TestOpenGame(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla, dora := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla"), register(t, s, "dora")

	g := x01Game(anna.PlayerID)
	g.Open, g.Seats = true, 3
	v, err := s.CreateGame(ctx, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Ready {
		t.Fatal("a game with free seats is ready")
	}
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); !errors.Is(err, ErrNotReady) {
		t.Errorf("dart while seats are free: got %v, want ErrNotReady", err)
	}

	// Strangers see it, its host doesn't, and a game that isn't open stays hidden.
	closed, _ := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil)
	if l, err := s.OpenLobbies(ctx, ben); err != nil || len(l) != 1 || l[0].ID != v.Game.ID || l[0].Host != "anna" || l[0].Players != 1 || l[0].Seats != 3 {
		t.Fatalf("ben's open games = %+v, %v; want anna's game with 1 of 3", l, err)
	}
	if l, _ := s.OpenLobbies(ctx, anna); len(l) != 0 {
		t.Errorf("anna sees her own game: %+v", l)
	}
	if _, err := s.JoinGame(ctx, ben, closed.Game.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("join a game that isn't open: got %v, want ErrNotFound", err)
	}

	if _, err := s.JoinGame(ctx, anna, v.Game.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("anna joins her own game: got %v, want ErrInvalid", err)
	}
	if v, err = s.JoinGame(ctx, ben, v.Game.ID); err != nil || v.Ready || !v.CanThrow {
		t.Fatalf("ben joins: ready %v, can throw %v, %v; want waiting, may throw", v.Ready, v.CanThrow, err)
	}
	if _, err := s.JoinGame(ctx, ben, v.Game.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben joins twice: got %v, want ErrInvalid", err)
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 0 {
		t.Errorf("ben still sees the game he is in: %+v", l)
	}

	if v, err = s.JoinGame(ctx, carla, v.Game.ID); err != nil || !v.Ready || len(v.Game.PlayerIDs) != 3 {
		t.Fatalf("carla takes the last seat: ready %v, players %v, %v; want ready with 3", v.Ready, v.Game.PlayerIDs, err)
	}
	if _, err := s.JoinGame(ctx, dora, v.Game.ID); !errors.Is(err, ErrNoSeat) {
		t.Errorf("join a full game: got %v, want ErrNoSeat", err)
	}
	if l, _ := s.OpenLobbies(ctx, dora); len(l) != 0 {
		t.Errorf("a full game is still offered: %+v", l)
	}
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); err != nil {
		t.Errorf("first dart of the full game: %v", err)
	}
}

func TestOpenGameValidation(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")

	for name, tt := range map[string]game.Lobby{
		"open without seats":    {Open: true},
		"no seat is free":       {Open: true, Seats: 1},
		"too many seats":        {Open: true, Seats: game.MaxSeats + 1},
		"seats without opening": {Seats: 3},
	} {
		g := x01Game(anna.PlayerID)
		g.Lobby = tt
		if _, err := s.CreateGame(ctx, anna, g, nil); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestOfferedOnlyForAWhile(t *testing.T) {
	open := game.Lobby{Open: true, Seats: 2}
	if !offered(open, 1, time.Now(), false) {
		t.Error("a new open game isn't offered")
	}
	if offered(open, 1, time.Now().Add(-OpenFor-time.Minute), false) {
		t.Error("an old open game is still offered")
	}
	if offered(open, 2, time.Now(), false) || offered(open, 1, time.Now(), true) || offered(game.Lobby{}, 1, time.Now(), false) {
		t.Error("a full, aborted or closed game is offered")
	}
}

func TestOpenTournament(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")

	tr := quickTournament(tournament.RoundRobin, anna.PlayerID)
	tr.Open, tr.Seats = true, 3
	v, err := s.CreateTournament(ctx, anna, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Ready || len(v.State.Matches) != 0 {
		t.Fatalf("ready %v with %d matches before the seats are taken", v.Ready, len(v.State.Matches))
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 1 || !l[0].Tournament || l[0].Format != tournament.RoundRobin {
		t.Fatalf("ben's open games = %+v, want the tournament", l)
	}
	if v, err = s.JoinTournament(ctx, ben, v.Tournament.ID); err != nil || v.Ready {
		t.Fatalf("ben joins: ready %v, %v", v.Ready, err)
	}
	if v, err = s.JoinTournament(ctx, carla, v.Tournament.ID); err != nil || !v.Ready || len(v.State.Matches) != 3 || len(v.Tournament.PlayerIDs) != 3 {
		t.Fatalf("carla takes the last seat: ready %v, %d matches, players %v, %v; want ready with 3 matches", v.Ready, len(v.State.Matches), v.Tournament.PlayerIDs, err)
	}
	if _, err := s.JoinTournament(ctx, ben, v.Tournament.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("join twice: got %v, want ErrInvalid", err)
	}
	if _, err := s.StartMatch(ctx, anna, v.Tournament.ID, 0); err != nil {
		t.Errorf("start the first match: %v", err)
	}
}
