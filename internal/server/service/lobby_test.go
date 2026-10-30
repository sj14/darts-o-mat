package service

import (
	"context"
	"errors"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

func TestOpenGame(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla, dora := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla"), register(t, s, "dora")

	g := x01Game(anna.PlayerID)
	g.Seats = 3
	g.Settings.TurnSeconds = 60
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Started {
		t.Fatal("a game with free seats is ready")
	}
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); !errors.Is(err, ErrNotReady) {
		t.Errorf("dart while seats are free: got %v, want ErrNotReady", err)
	}

	// Strangers see it with the players who are in, those in it don't, and a game that isn't open stays hidden.
	closed, _ := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil)
	if l, err := s.OpenLobbies(ctx, ben); err != nil || len(l) != 1 || l[0].ID != v.Game.ID || len(l[0].Joined) != 1 || l[0].Joined[0].Name != "anna" || l[0].Seats != 3 {
		t.Fatalf("ben's open games = %+v, %v; want anna's game with 1 of 3", l, err)
	}
	if l, _ := s.OpenLobbies(ctx, anna); len(l) != 0 {
		t.Errorf("anna sees her own game: %+v", l)
	}
	if _, err := joinGame(ctx, s, ben, closed.Game.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("join a game that isn't open: got %v, want ErrNotFound", err)
	}

	if _, err := joinGame(ctx, s, anna, v.Game.ID, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("anna joins her own game: got %v, want ErrInvalid", err)
	}
	if v, err = joinGame(ctx, s, ben, v.Game.ID, nil); err != nil || v.Started || !v.CanThrow {
		t.Fatalf("ben joins: ready %v, can throw %v, %v; want waiting, may throw", v.Started, v.CanThrow, err)
	}
	if _, err := joinGame(ctx, s, ben, v.Game.ID, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben joins twice: got %v, want ErrInvalid", err)
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 0 {
		t.Errorf("ben still sees the game he is in: %+v", l)
	}

	if v, err = joinGame(ctx, s, carla, v.Game.ID, nil); err != nil || !v.Started || len(v.Game.PlayerIDs()) != 3 {
		t.Fatalf("carla takes the last seat: ready %v, players %v, %v; want ready with 3", v.Started, v.Game.PlayerIDs(), err)
	}
	if _, err := joinGame(ctx, s, dora, v.Game.ID, nil); !errors.Is(err, ErrNoSeat) {
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
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")

	for name, tt := range map[string]game.Lobby{
		"no seat is free":      {Seats: 1},
		"too many seats":       {Seats: game.MaxSeats + 1},
		"negative seats":       {Seats: -1},
		"board only, not open": {BoardOnly: true},
		"no time per turn":     {Seats: 2},
	} {
		g := x01Game(anna.PlayerID)
		g.Lobby = tt
		if _, err := createGame(ctx, s, anna, g, nil); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestOffered(t *testing.T) {
	t.Parallel()
	open := game.Lobby{Seats: 2}
	if !offered(open, 1, false) {
		t.Error("an open game with a free seat isn't offered")
	}
	if offered(open, 2, false) || offered(open, 1, true) || offered(game.Lobby{}, 1, false) {
		t.Error("a full, aborted or closed game is offered")
	}
}

func TestOpenTournament(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")

	tr := quickTournament(tournament.RoundRobin, anna.PlayerID)
	tr.Seats = 3
	tr.Settings.TurnSeconds = 60
	v, err := createTournament(ctx, s, anna, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Started || len(v.State.Matches) != 0 {
		t.Fatalf("ready %v with %d matches before the seats are taken", v.Started, len(v.State.Matches))
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 1 || !l[0].Tournament || l[0].Format != tournament.RoundRobin {
		t.Fatalf("ben's open games = %+v, want the tournament", l)
	}
	if v, err = joinTournament(ctx, s, ben, v.Tournament.ID, nil); err != nil || v.Started {
		t.Fatalf("ben joins: ready %v, %v", v.Started, err)
	}
	if v, err = joinTournament(ctx, s, carla, v.Tournament.ID, nil); err != nil || !v.Started || len(v.State.Matches) != 3 || len(v.Tournament.PlayerIDs()) != 3 {
		t.Fatalf("carla takes the last seat: ready %v, %d matches, players %v, %v; want ready with 3 matches", v.Started, len(v.State.Matches), v.Tournament.PlayerIDs(), err)
	}
	if _, err := joinTournament(ctx, s, ben, v.Tournament.ID, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("join twice: got %v, want ErrInvalid", err)
	}
	// Whoever has a player in a match may start it: the first match anna plays.
	for _, m := range v.State.Matches {
		if m.PlayerIDs[0] == anna.PlayerID || m.PlayerIDs[1] == anna.PlayerID {
			if _, err := s.StartMatch(ctx, anna, v.Tournament.ID, m.Index); err != nil {
				t.Errorf("start match %d: %v", m.Index, err)
			}
			break
		}
	}
}

// Leaving a tournament that hasn't started takes the player out and shows nothing; leaving a
// running one shows it, aborted.
func TestLeaveTournamentViews(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")

	tr := quickTournament(tournament.RoundRobin, anna.PlayerID)
	tr.Seats = 3
	tr.Settings.TurnSeconds = 60
	v, _ := createTournament(ctx, s, anna, tr, nil)
	id := v.Tournament.ID
	_, _ = joinTournament(ctx, s, ben, id, nil)
	lobby, err := s.LeaveTournament(ctx, ben, id)
	if err != nil || lobby.Tournament.ID != 0 {
		t.Fatalf("leave the lobby: %+v, %v; want an empty view", lobby.Tournament.ID, err)
	}

	_, _ = joinTournament(ctx, s, ben, id, nil)
	if _, err := joinTournament(ctx, s, carla, id, nil); err != nil {
		t.Fatal(err)
	}
	running, err := s.LeaveTournament(ctx, carla, id)
	if err != nil || !running.Tournament.Aborted() {
		t.Errorf("leave a running tournament: aborted %v, %v; want it aborted and shown", running.Tournament.Aborted(), err)
	}
	// Only carla, who left, has it in her statistics; leaving the lobby earlier counted for nobody.
	for name, want := range map[string]int{"anna": 0, "ben": 0, "carla": 1} {
		acc := map[string]account.Account{"anna": anna, "ben": ben, "carla": carla}[name]
		if got, _ := s.Stats(ctx, acc, ""); got[0].X01.TournamentsLeft != want || got[0].X01.GamesLeft != 0 {
			t.Errorf("%s left %+v, want %d tournaments", name, got[0].X01, want)
		}
	}
}

func TestBoardOnlyGame(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	annas, _ := s.CreateBoard(ctx, anna, "Annas")
	bens, _ := s.CreateBoard(ctx, ben, "Bens")

	g := x01Game(anna.PlayerID)
	g.Lobby = game.Lobby{Seats: 2, BoardOnly: true}
	g.Settings.TurnSeconds = 60
	if _, err := createGame(ctx, s, anna, g, nil); !errors.Is(err, ErrBoardRequired) {
		t.Fatalf("create without a board: got %v, want ErrBoardRequired", err)
	}
	v, err := createGame(ctx, s, anna, withBoard(g, annas.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 1 || !l[0].BoardOnly {
		t.Fatalf("ben's open games = %+v, want the one that needs a board", l)
	}
	if _, err := joinGame(ctx, s, ben, v.Game.ID, nil); !errors.Is(err, ErrBoardRequired) {
		t.Errorf("join without a board: got %v, want ErrBoardRequired", err)
	}
	if _, err := s.UseBoard(ctx, anna, v.Game.ID, nil); !errors.Is(err, ErrBoardRequired) {
		t.Errorf("take the board away: got %v, want ErrBoardRequired", err)
	}
	if v, err = joinGame(ctx, s, ben, v.Game.ID, &bens.ID); err != nil || !v.Started {
		t.Fatalf("join with a board: ready %v, %v; want ready", v.Started, err)
	}

	// Running, the game keeps its boards: a newer game (of a guest, anna plays already)
	// cannot take one.
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	if _, err := createGame(ctx, s, anna, withBoard(x01Game(papa.ID), annas.ID), nil); !errors.Is(err, ErrBoardBusy) {
		t.Errorf("new game on a board in a board-only game: got %v, want ErrBoardBusy", err)
	}
	if g, err := createGame(ctx, s, anna, x01Game(papa.ID), nil); err != nil {
		t.Errorf("new game without a board: %v", err)
	} else if _, err := s.UseBoard(ctx, anna, g.Game.ID, &annas.ID); !errors.Is(err, ErrBoardBusy) {
		t.Errorf("move another game to the board: got %v, want ErrBoardBusy", err)
	}
}

// Whoever takes no part in a game learns nothing about it, not even that it needs a
// board, and a refused request changes nothing of theirs.
func TestOutsiderIsRefusedFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, carl := register(t, s, "anna"), register(t, s, "carl")
	annas, _ := s.CreateBoard(ctx, anna, "Annas")
	g := withBoard(x01Game(anna.PlayerID), annas.ID)
	g.Lobby = game.Lobby{Seats: 2, BoardOnly: true}
	g.Settings.TurnSeconds = 60
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}

	// carl just finished a game of his own: it can still be corrected.
	own, _ := createGame(ctx, s, carl, quick(carl.PlayerID), nil)
	if _, err := s.AddDart(ctx, carl, own.Game.ID, d20); err != nil {
		t.Fatal(err)
	}

	if _, err := s.UseBoard(ctx, carl, v.Game.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("an outsider takes a board away: got %v, want ErrNotFound", err)
	}
	if _, err := acceptGame(ctx, s, carl, v.Game.ID, &annas.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("an outsider accepts without an invitation: got %v, want ErrNotFound", err)
	}
	if own, _ = s.Game(ctx, carl, own.Game.ID); own.Completed {
		t.Error("a refused accept completed the outsider's own game")
	}
}

// The list of open games shows the form of the players in each game's mode, from their
// stored results, for those with enough games of it.
func TestOpenLobbyForms(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")

	var results []stats.Result
	for id := int64(1000); id < 1003; id++ { // anna: 3 games of X01 at 60, ben only 2
		results = append(results,
			stats.Result{GameID: id, PlayerID: anna.PlayerID, Mode: game.ModeX01, Darts: 15, Points: 300},
			stats.Result{GameID: id + 10, PlayerID: anna.PlayerID, Mode: game.ModeAroundTheClock, Darts: 10, Hits: 5})
		if id < 1002 {
			results = append(results, stats.Result{GameID: id, PlayerID: ben.PlayerID, Mode: game.ModeX01, Darts: 15, Points: 300})
		}
	}
	if err := s.store.AddResults(ctx, results); err != nil {
		t.Fatal(err)
	}

	g := x01Game(anna.PlayerID)
	g.Seats = 3
	g.Settings.TurnSeconds = 60
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := joinGame(ctx, s, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	l, err := s.OpenLobbies(ctx, carla)
	if err != nil || len(l) != 1 {
		t.Fatalf("carla's open games = %+v, %v; want anna's", l, err)
	}
	if form, ok := l[0].Forms[anna.PlayerID]; !ok || form != 60 {
		t.Errorf("anna's form = %v, %v; want her X01 average 60, not around the clock", form, ok)
	}
	if form, ok := l[0].Forms[ben.PlayerID]; ok {
		t.Errorf("ben has a form with 2 games: %v", form)
	}
}

// A lobby is only offered while someone who is in it is online: strangers don't see one
// whose players left, can't join it and a quick match doesn't take it, until they are back.
func TestOpenLobbyWaiting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	anna, ben, carla, dora := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla"), register(t, s, "dora")

	g := x01Game(anna.PlayerID)
	g.Seats = 3
	g.Settings.TurnSeconds = 60
	annas, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	tr := quickTournament(tournament.RoundRobin, ben.PlayerID)
	tr.Seats = 3
	tr.Settings.TurnSeconds = 60
	bens, err := createTournament(ctx, s, ben, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	carlas, err := s.QuickMatch(ctx, carla, game.ModeX01, nil)
	if err != nil {
		t.Fatal(err)
	}
	if l, err := s.OpenLobbies(ctx, dora); err != nil || len(l) != 3 {
		t.Fatalf("dora's open games while the others are online = %+v, %v; want 3", l, err)
	}

	offset = onlineFor
	if l, err := s.OpenLobbies(ctx, dora); err != nil || len(l) != 0 {
		t.Errorf("dora's open games after the others went = %+v, %v; want none", l, err)
	}
	if _, err := joinGame(ctx, s, dora, annas.Game.ID, nil); !errors.Is(err, ErrNoSeat) {
		t.Errorf("join a game nobody waits in: got %v, want ErrNoSeat", err)
	}
	if _, err := joinTournament(ctx, s, dora, bens.Tournament.ID, nil); !errors.Is(err, ErrNoSeat) {
		t.Errorf("join a tournament nobody waits in: got %v, want ErrNoSeat", err)
	}
	doras, err := s.QuickMatch(ctx, dora, game.ModeX01, nil)
	if err != nil || doras.Game.ID == carlas.Game.ID || doras.Started {
		t.Errorf("dora's quick match: game %d started %v, %v; want one of her own, waiting", doras.Game.ID, doras.Started, err)
	}

	s.seen(ctx, carla)
	if moved, _, err := s.QuickMatchMove(ctx, dora, doras.Game.ID); err != nil || moved != carlas.Game.ID {
		t.Errorf("dora moves to %d, %v; want carla's quick match once she is back", moved, err)
	}
}
