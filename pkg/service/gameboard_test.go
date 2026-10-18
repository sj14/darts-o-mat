package service

import (
	"context"
	"errors"
	"testing"
)

func TestUseBoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	keller, err := s.CreateBoard(ctx, anna, "Keller")
	if err != nil {
		t.Fatal(err)
	}
	bens, _ := s.CreateBoard(ctx, ben, "Bens")

	// A game without a board offers the owner's boards; none is in use yet.
	first, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Boards) != 1 || first.Boards[0].ID != keller.ID || first.Boards[0].InUse {
		t.Fatalf("choices = %+v, want the free Keller", first.Boards)
	}

	if _, err := s.UseBoard(ctx, ben, first.Game.ID, bens.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben uses a board in anna's game: %v, want ErrNotFound", err)
	}
	if _, err := s.UseBoard(ctx, anna, first.Game.ID, bens.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("anna uses ben's board: %v, want ErrInvalid", err)
	}

	v, err := s.UseBoard(ctx, anna, first.Game.ID, keller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Board == nil || v.Board.ID != keller.ID || len(v.Boards) != 0 {
		t.Errorf("board = %+v, choices = %+v, want Keller and no choices", v.Board, v.Boards)
	}
	if boards, _ := s.Boards(ctx, anna); boards[0].LastUsedAt == nil {
		t.Error("the board isn't marked as used")
	}
	if _, err := s.UseBoard(ctx, anna, first.Game.ID, keller.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("a game that has a board already: %v, want ErrInvalid", err)
	}

	// A second game takes the board over; the first continues without one.
	second, _ := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil)
	if len(second.Boards) != 1 || !second.Boards[0].InUse {
		t.Fatalf("choices = %+v, want Keller in use", second.Boards)
	}
	if v, err = s.UseBoard(ctx, anna, second.Game.ID, keller.ID); err != nil || v.Board == nil {
		t.Fatalf("second game takes the board: %v, %+v", err, v.Board)
	}
	if v, _ = s.Game(ctx, anna, first.Game.ID); v.Board != nil {
		t.Errorf("first game still has the board %+v", v.Board)
	}
	if _, g, err := s.activeGame(ctx, keller); err != nil || g.Game.ID != second.Game.ID {
		t.Errorf("camera darts go to game %d (%v), want %d", g.Game.ID, err, second.Game.ID)
	}
}

func TestNewGameTakesTheBoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")

	onBoard := x01Game(anna.PlayerID)
	onBoard.BoardID = &keller.ID
	first, err := s.CreateGame(ctx, anna, onBoard, nil)
	if err != nil {
		t.Fatal(err)
	}
	if boards, _ := s.Boards(ctx, anna); !boards[0].InUse {
		t.Error("the board isn't in use by the first game")
	}

	onBoard = x01Game(anna.PlayerID)
	onBoard.BoardID = &keller.ID
	second, err := s.CreateGame(ctx, anna, onBoard, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Game(ctx, anna, first.Game.ID); v.Board != nil {
		t.Errorf("first game still has the board %+v", v.Board)
	}
	if v, _ := s.Game(ctx, anna, second.Game.ID); v.Board == nil {
		t.Error("second game has no board")
	}
	if _, g, err := s.activeGame(ctx, keller); err != nil || g.Game.ID != second.Game.ID {
		t.Errorf("camera darts go to game %d (%v), want %d", g.Game.ID, err, second.Game.ID)
	}
}
