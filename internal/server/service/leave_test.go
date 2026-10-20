package service

import (
	"context"
	"errors"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
)

func TestLeaveGame(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)

	v, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben", "carla"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	if _, err := s.LeaveGame(ctx, ben, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben declines by leaving before he accepted: got %v, want ErrForbidden", err)
	}
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AcceptInvitation(ctx, carla, id, nil)

	for range 3 { // anna's visit
		if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
			t.Fatal(err)
		}
	}
	if v, err = s.AddDart(ctx, ben, id, t20); err != nil {
		t.Fatal(err)
	}
	for _, d := range v.Darts {
		if d.PlayerID == 0 {
			t.Fatalf("a dart without its player: %+v", d)
		}
	}
	if v.Darts[0].PlayerID != anna.PlayerID || v.Darts[3].PlayerID != ben.PlayerID {
		t.Errorf("thrown by %d and %d, want anna %d and ben %d", v.Darts[0].PlayerID, v.Darts[3].PlayerID, anna.PlayerID, ben.PlayerID)
	}

	// Ben leaves while up: carla throws next, and ben can look at the game only.
	v, err = s.LeaveGame(ctx, ben, id)
	if err != nil {
		t.Fatal(err)
	}
	if !v.HasLeft || v.CanThrow || v.Game.Aborted() {
		t.Fatalf("ben after leaving: left %v, can throw %v, aborted %v; want left, no throwing, game goes on", v.HasLeft, v.CanThrow, v.Game.Aborted())
	}
	if cur := v.Common().CurrentPlayerID; cur == nil || *cur != carla.PlayerID {
		t.Fatalf("current player %v, want carla", cur)
	}
	if _, err := s.AddDart(ctx, ben, id, t20); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben throws after leaving: got %v, want ErrForbidden", err)
	}
	if _, err := s.LeaveGame(ctx, ben, id); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben leaves twice: got %v, want ErrInvalid", err)
	}
	if _, err := s.UndoDart(ctx, anna, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("anna takes back ben's leaving: got %v, want ErrForbidden", err)
	}
	// The others go round without him.
	for range 3 {
		if _, err := s.AddDart(ctx, carla, id, t20); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ = s.Game(ctx, anna, id); *v.Common().CurrentPlayerID != anna.PlayerID {
		t.Errorf("after carla: current player %d, want anna", *v.Common().CurrentPlayerID)
	}
	if left := v.Common().Records[ben.PlayerID]; !left.Left {
		t.Error("ben isn't marked as having left")
	}

	// With only two players left, leaving ends the game.
	if v, err = s.LeaveGame(ctx, carla, id); err != nil || !v.Game.Aborted() {
		t.Errorf("carla leaves the last two: aborted %v, %v; want the game aborted", v.Game.Aborted(), err)
	}
}

func TestLeaveTheLobby(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	g := x01Game(anna.PlayerID)
	g.Seats = 2
	v, _ := s.CreateGame(ctx, anna, g, nil)
	if _, err := s.JoinGame(ctx, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	// The game started with the last seat; make it wait for a third player instead.
	g.Seats = 3
	v, _ = s.CreateGame(ctx, anna, g, nil)
	if _, err := s.JoinGame(ctx, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	gv, err := s.LeaveGame(ctx, ben, v.Game.ID)
	if err != nil || gv.Game.ID != 0 {
		t.Fatalf("leave the lobby: %+v, %v; want an empty view", gv.Game.ID, err)
	}
	if l, _ := s.OpenLobbies(ctx, ben); len(l) != 1 || len(l[0].Joined) != 1 {
		t.Errorf("open games for ben = %+v, want the game with its seat free again", l)
	}
	if _, err := s.Game(ctx, ben, v.Game.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben looks at a game he left: got %v, want ErrNotFound", err)
	}
}

func TestCantLeaveWhileBullingOff(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)
	g := x01Game(anna.PlayerID)
	g.BullOff = true
	v, _ := s.CreateGame(ctx, anna, g, []string{"ben", "carla"})
	_, _ = s.AcceptInvitation(ctx, ben, v.Game.ID, nil)
	_, _ = s.AcceptInvitation(ctx, carla, v.Game.ID, nil)

	if _, err := s.LeaveGame(ctx, ben, v.Game.ID); !errors.Is(err, turn.ErrCantLeave) {
		t.Errorf("leave while bulling off: got %v, want ErrCantLeave", err)
	}
}

// Nobody is the host: whoever created the game can leave it like the others.
func TestTheCreatorCanLeaveToo(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	v, _ := s.CreateGame(ctx, anna, x01Game(anna.PlayerID, papa.ID), []string{"ben", "carla"})
	_, _ = s.AcceptInvitation(ctx, ben, v.Game.ID, nil)
	_, _ = s.AcceptInvitation(ctx, carla, v.Game.ID, nil)

	// Anna leaves with her guest: ben and carla play on.
	v, err := s.LeaveGame(ctx, anna, v.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.Game.Aborted() || !v.HasLeft || !v.Common().Records[papa.ID].Left || !v.Common().Records[anna.PlayerID].Left {
		t.Fatalf("after anna left: aborted %v, left %v, papa left %v; want both gone and the game on", v.Game.Aborted(), v.HasLeft, v.Common().Records[papa.ID].Left)
	}
	if cur := v.Common().CurrentPlayerID; cur == nil || *cur != ben.PlayerID {
		t.Errorf("current player %v, want ben", cur)
	}
	if _, err := s.AddDart(ctx, ben, v.Game.ID, t20); err != nil {
		t.Errorf("ben throws: %v", err)
	}
}

func TestLeavingCountsForWhoLeft(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)
	v, _ := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben", "carla"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AcceptInvitation(ctx, carla, id, nil)

	left := func(acc account.Account) int {
		got, err := s.Stats(ctx, acc, "")
		if err != nil {
			t.Fatal(err)
		}
		return got[0].X01.GamesLeft
	}
	if _, err := s.LeaveGame(ctx, ben, id); err != nil { // the others play on
		t.Fatal(err)
	}
	if _, err := s.LeaveGame(ctx, carla, id); err != nil { // anna alone: the game is aborted
		t.Fatal(err)
	}
	if a, b, c := left(anna), left(ben), left(carla); a != 0 || b != 1 || c != 1 {
		t.Errorf("games left: anna %d, ben %d, carla %d; want 0, 1, 1", a, b, c)
	}
}
