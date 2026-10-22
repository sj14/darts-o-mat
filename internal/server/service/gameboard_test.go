package service

import (
	"context"
	"errors"
	"testing"
)

// onBoard reports whether the player throws on the board in the game.
func onBoard(v GameView, playerID, boardID int64) bool {
	b := v.BoardOf(playerID)
	return b != nil && *b == boardID
}

func TestUseBoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	keller, err := s.CreateBoard(ctx, anna, "Keller")
	if err != nil {
		t.Fatal(err)
	}
	bens, _ := s.CreateBoard(ctx, ben, "Bens")

	// A game without a board offers the viewer's boards; none is in use yet.
	first, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Boards) != 1 || first.Boards[0].ID != keller.ID || first.Boards[0].InUse {
		t.Fatalf("choices = %+v, want the free Keller", first.Boards)
	}

	if _, err := s.UseBoard(ctx, ben, first.Game.ID, &bens.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben uses a board in anna's game: %v, want ErrNotFound", err)
	}
	if _, err := s.UseBoard(ctx, anna, first.Game.ID, &bens.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("anna uses ben's board: %v, want ErrInvalid", err)
	}

	v, err := s.UseBoard(ctx, anna, first.Game.ID, &keller.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !onBoard(v, anna.PlayerID, keller.ID) || len(v.Boards) != 1 {
		t.Errorf("board of anna = %v, choices = %+v, want Keller and the choice to change", v.BoardOf(anna.PlayerID), v.Boards)
	}
	if boards, _ := s.Boards(ctx, anna); boards[0].LastUsedAt == nil {
		t.Error("the board isn't marked as used")
	}

	// A second game, of a guest, takes the board over; the first continues without it.
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	second, err := createGame(ctx, s, anna, x01Game(papa.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Boards) != 1 || !second.Boards[0].InUse {
		t.Fatalf("choices = %+v, want Keller in use", second.Boards)
	}
	if v, err = s.UseBoard(ctx, anna, second.Game.ID, &keller.ID); err != nil || !onBoard(v, papa.ID, keller.ID) {
		t.Fatalf("second game takes the board: %v", err)
	}
	if v, _ = s.Game(ctx, anna, first.Game.ID); v.HasBoard() {
		t.Errorf("first game still has a board: %v", v.BoardOf(anna.PlayerID))
	}
	if g, err := s.activeGame(ctx, keller); err != nil || g.Game.ID != second.Game.ID {
		t.Errorf("camera darts go to game %d (%v), want %d", g.Game.ID, err, second.Game.ID)
	}
}

func TestNewGameTakesTheBoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")

	first, err := createGame(ctx, s, anna, withBoard(x01Game(anna.PlayerID), keller.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if boards, _ := s.Boards(ctx, anna); !boards[0].InUse {
		t.Error("the board isn't in use by the first game")
	}

	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	second, err := createGame(ctx, s, anna, withBoard(x01Game(papa.ID), keller.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Game(ctx, anna, first.Game.ID); v.HasBoard() {
		t.Error("first game still has the board")
	}
	if v, _ := s.Game(ctx, anna, second.Game.ID); !v.HasBoard() {
		t.Error("second game has no board")
	}
	if g, err := s.activeGame(ctx, keller); err != nil || g.Game.ID != second.Game.ID {
		t.Errorf("camera darts go to game %d (%v), want %d", g.Game.ID, err, second.Game.ID)
	}
}

// Friends at one board share it: the friend picks the board the host throws on.
func TestFriendsShareABoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	bens, _ := s.CreateBoard(ctx, ben, "Bens")

	v, err := createGame(ctx, s, anna, withBoard(x01Game(anna.PlayerID), keller.ID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Boards) != 1 || v.Boards[0].ID != keller.ID {
		t.Errorf("anna's choices = %+v, want her Keller, to change or leave it", v.Boards)
	}
	bv, _ := s.Game(ctx, ben, v.Game.ID)
	if len(bv.Boards) != 2 || !bv.Boards[1].Others || bv.Boards[1].ID != keller.ID {
		t.Fatalf("ben's choices = %+v, want his own board and anna's", bv.Boards)
	}
	if _, err := acceptGame(ctx, s, ben, v.Game.ID, &keller.ID); err != nil {
		t.Fatalf("ben accepts on anna's board: %v", err)
	}
	if _, err := acceptGame(ctx, s, ben, v.Game.ID, &bens.ID); err == nil {
		t.Error("accepting twice worked")
	}
	if v, _ = s.Game(ctx, ben, v.Game.ID); !onBoard(v, ben.PlayerID, keller.ID) || !onBoard(v, anna.PlayerID, keller.ID) {
		t.Error("both don't throw on Keller")
	}
	// The one camera reports the darts of both.
	for range 3 {
		if _, err := s.AddBoardDart(ctx, keller, t20); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddBoardDart(ctx, keller, t20); err != nil { // implies the takeout, ben's dart
		t.Errorf("a dart of ben through anna's camera: %v", err)
	}
}

// A board that broke is taken from the running game: its players enter the darts by hand
// from then on, as if they had played without a board from the start.
func TestUseNoBoard(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	keller, _ := s.CreateBoard(ctx, anna, "Keller")

	v, err := createGame(ctx, s, anna, withBoard(x01Game(anna.PlayerID), keller.ID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acceptGame(ctx, s, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, anna, v.Game.ID); !v.HasBoard() || !v.HasCamera(anna.PlayerID) {
		t.Fatal("the game starts on the board")
	}

	// ben isn't on a board and must not take anna's players off theirs.
	if v, err = s.UseBoard(ctx, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	if !v.HasCamera(anna.PlayerID) {
		t.Error("ben took anna off her board")
	}

	if v, err = s.UseBoard(ctx, anna, v.Game.ID, nil); err != nil {
		t.Fatalf("anna takes her players off the board: %v", err)
	}
	if v.HasCamera(anna.PlayerID) || v.HasBoard() {
		t.Errorf("anna's board = %v, the game has a board: %v; want neither", v.BoardOf(anna.PlayerID), v.HasBoard())
	}
	// The board is free again and not used by the game.
	if boards, _ := s.Boards(ctx, anna); boards[0].InUse {
		t.Error("the board is still in use")
	}
	if _, err := s.activeGame(ctx, keller); err == nil {
		t.Error("the board's camera still reaches the game")
	}
	// Darts are entered by hand now.
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); err != nil {
		t.Errorf("a dart by hand: %v", err)
	}
	// She can go back to the board.
	if v, err = s.UseBoard(ctx, anna, v.Game.ID, &keller.ID); err != nil || !onBoard(v, anna.PlayerID, keller.ID) {
		t.Errorf("back to the board: %v", err)
	}
}
