package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
)

func TestPlayersOnTheirOwnBoards(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	boardA, _ := s.CreateBoard(ctx, anna, "A")
	boardB, _ := s.CreateBoard(ctx, ben, "B")

	g := x01Game(anna.PlayerID)
	g = withBoard(g, boardA.ID)
	g.Open, g.Seats = true, 2
	v, err := s.CreateGame(ctx, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	carla := register(t, s, "carla")
	boardC, _ := s.CreateBoard(ctx, carla, "C")
	if _, err := s.JoinGame(ctx, ben, v.Game.ID, &boardC.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("ben joins on carla's board: got %v, want ErrInvalid", err)
	}
	if v, err = s.JoinGame(ctx, ben, v.Game.ID, &boardB.ID); err != nil || !v.Ready || !v.HasBoard() {
		t.Fatalf("ben joins on his board: ready %v, has a board %v, %v", v.Ready, v.HasBoard(), err)
	}
	id := v.Game.ID
	if len(v.BoardNames) != 2 || v.BoardNames[ben.PlayerID] != "B" || v.BoardNames[anna.PlayerID] != "A" {
		t.Errorf("board names = %v, want anna's A and ben's B", v.BoardNames)
	}

	// Each board only counts the darts of its player's turns.
	for range 3 {
		if _, err := s.AddBoardDart(ctx, boardA, t20); err != nil {
			t.Fatalf("anna's dart on her board: %v", err)
		}
	}
	if _, err := s.AddBoardDart(ctx, boardB, t20); err != nil {
		t.Fatalf("ben's first dart implies the takeout of anna's turn: %v", err)
	}
	if _, err := s.AddBoardDart(ctx, boardA, t20); !errors.Is(err, ErrNotOnBoard) {
		t.Errorf("anna's board during ben's turn: got %v, want ErrNotOnBoard", err)
	}
	if v, _ = s.BoardTakeout(ctx, boardA); len(v.Darts) != 5 {
		t.Errorf("takeout of the wrong board changed the log: %d entries, want 5 (3 darts, takeout, dart)", len(v.Darts))
	}
	for range 2 {
		if _, err := s.AddBoardDart(ctx, boardB, t20); err != nil {
			t.Fatal(err)
		}
	}
	if v, err = s.BoardTakeout(ctx, boardB); err != nil || v.Common().CurrentPlayerID == nil || *v.Common().CurrentPlayerID != anna.PlayerID {
		t.Fatalf("ben's takeout: %v, current %v; want anna up", err, v.Common().CurrentPlayerID)
	}

	// A board belongs to one game: ben starts another on his board.
	other := x01Game(ben.PlayerID)
	other = withBoard(other, boardB.ID)
	if _, err := s.CreateGame(ctx, ben, other, nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, id); v.BoardOf(ben.PlayerID) != nil && *v.BoardOf(ben.PlayerID) == boardB.ID {
		t.Error("ben's board is still in the first game")
	}
}

func TestOpenTournamentOnTheirOwnBoards(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	boardB, _ := s.CreateBoard(ctx, ben, "B")
	boardC, _ := s.CreateBoard(ctx, carla, "C")

	tr := quickTournament(tournament.RoundRobin, anna.PlayerID)
	tr.Open, tr.Seats = true, 3
	v, err := s.CreateTournament(ctx, anna, tr, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Tournament.ID
	if _, err := s.JoinTournament(ctx, ben, id, &boardB.ID); err != nil {
		t.Fatal(err)
	}
	if v, err = s.JoinTournament(ctx, carla, id, &boardC.ID); err != nil || !v.Ready {
		t.Fatalf("carla joins: ready %v, %v", v.Ready, err)
	}

	// The match between ben and carla is played on their boards.
	var match int
	for _, m := range v.State.Matches {
		if m.PlayerIDs[0] != anna.PlayerID && m.PlayerIDs[1] != anna.PlayerID {
			match = m.Index
		}
	}
	gv, err := s.StartMatch(ctx, ben, id, match)
	if err != nil {
		t.Fatal(err)
	}
	if !gv.HasBoard() || gv.BoardOf(ben.PlayerID) == nil || *gv.BoardOf(ben.PlayerID) != boardB.ID || *gv.BoardOf(carla.PlayerID) != boardC.ID {
		t.Fatalf("boards of the match: ben %v, carla %v, want B and C", gv.BoardOf(ben.PlayerID), gv.BoardOf(carla.PlayerID))
	}
	first, second := boardB, boardC
	if cur := gv.Common().CurrentPlayerID; cur != nil && *cur == carla.PlayerID {
		first, second = boardC, boardB
	}
	if _, err := s.AddBoardDart(ctx, second, t20); !errors.Is(err, ErrNotOnBoard) {
		t.Errorf("dart of the player who isn't up: got %v, want ErrNotOnBoard", err)
	}
	if _, err := s.AddBoardDart(ctx, first, t20); err != nil {
		t.Errorf("dart of the player who is up: %v", err)
	}
}
