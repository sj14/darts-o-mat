package service

import (
	"context"
	"errors"
	"testing"
)

func TestRatePlayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	guest, err := s.CreateGuest(ctx, anna, "Gast")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := createGame(ctx, s, anna, quick(anna.PlayerID, guest.ID), []string{"ben"})
	id := v.Game.ID
	_, _ = acceptGame(ctx, s, ben, id, nil)

	if _, err := s.RatePlayer(ctx, anna, id, ben.PlayerID, true); !errors.Is(err, ErrNotOver) {
		t.Errorf("rate while playing: got %v, want ErrNotOver", err)
	}
	if _, err := s.AddDart(ctx, anna, id, d20); err != nil {
		t.Fatal(err)
	}

	if _, err := s.RatePlayer(ctx, carla, id, ben.PlayerID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("a stranger rates: got %v, want ErrNotFound", err)
	}
	if _, err := s.RatePlayer(ctx, anna, id, anna.PlayerID, true); !errors.Is(err, ErrNotRateable) {
		t.Errorf("rate oneself: got %v, want ErrNotRateable", err)
	}
	if _, err := s.RatePlayer(ctx, ben, id, guest.ID, true); !errors.Is(err, ErrNotRateable) {
		t.Errorf("rate a guest: got %v, want ErrNotRateable", err)
	}
	if v, _ = s.Game(ctx, anna, id); len(v.Rateable()) != 1 || v.Rateable()[0].ID != ben.PlayerID {
		t.Errorf("anna can rate %+v, want only ben", v.Rateable())
	}

	if v, err = s.RatePlayer(ctx, anna, id, ben.PlayerID, true); err != nil || !v.Ratings[ben.PlayerID] {
		t.Fatalf("thumb up: %v, %v", v.Ratings, err)
	}
	v, err = s.RatePlayer(ctx, anna, id, ben.PlayerID, false)
	if up, ok := v.Ratings[ben.PlayerID]; err != nil || !ok || up {
		t.Fatalf("thumb down replaces it: %v, %v", v.Ratings, err)
	}
	got, _ := s.Stats(ctx, ben, "")
	if got[0].Reputation.Up != 0 || got[0].Reputation.Down != 1 {
		t.Errorf("ben's thumbs = %+v, want one down", got[0].Reputation)
	}
	if bv, _ := s.Game(ctx, ben, id); len(bv.Ratings) != 0 {
		t.Errorf("ben sees anna's thumbs: %v", bv.Ratings)
	}
}

// Others see the share of thumbs up in the list of open games, from three thumbs on.
func TestReputationInOpenGames(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	ben, erik := register(t, s, "ben"), register(t, s, "erik")
	lobby := x01Game(ben.PlayerID)
	lobby.Seats = 2
	for _, name := range []string{"anna", "carla", "dora"} {
		rater := register(t, s, name)
		befriend(t, s, rater, ben)
		v, _ := createGame(ctx, s, rater, quick(rater.PlayerID), []string{"ben"})
		_, _ = acceptGame(ctx, s, ben, v.Game.ID, nil)
		if _, err := s.AddDart(ctx, rater, v.Game.ID, d20); err != nil {
			t.Fatal(err)
		}
		up := name != "dora"
		if _, err := s.RatePlayer(ctx, rater, v.Game.ID, ben.PlayerID, up); err != nil {
			t.Fatal(err)
		}
		if name == "carla" { // two thumbs are too few to show
			v, err := createGame(ctx, s, ben, lobby, nil)
			if err != nil {
				t.Fatal(err)
			}
			if l, _ := s.OpenLobbies(ctx, erik); len(l) != 1 || len(l[0].Reputations) != 0 {
				t.Fatalf("with two thumbs: %+v, want none shown", l)
			}
			// Ben leaves it to play with dora.
			if _, err := s.LeaveGame(ctx, ben, v.Game.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := createGame(ctx, s, ben, lobby, nil); err != nil {
		t.Fatal(err)
	}
	l, _ := s.OpenLobbies(ctx, erik)
	if len(l) != 1 || l[0].Reputations[ben.PlayerID].Up != 2 || l[0].Reputations[ben.PlayerID].Down != 1 {
		t.Errorf("with three thumbs: %+v, want ben's 2 up and 1 down", l)
	}
}

// Many games against one player count as one thumb, the newest.
func TestThumbsCountOncePerPlayer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	for i, up := range []bool{true, true, false} {
		v, _ := createGame(ctx, s, anna, quick(anna.PlayerID), []string{"ben"})
		_, _ = acceptGame(ctx, s, ben, v.Game.ID, nil)
		if _, err := s.AddDart(ctx, anna, v.Game.ID, d20); err != nil {
			t.Fatal(err)
		}
		if _, err := s.RatePlayer(ctx, anna, v.Game.ID, ben.PlayerID, up); err != nil {
			t.Fatalf("game %d: %v", i, err)
		}
	}
	if got, _ := s.Stats(ctx, ben, ""); got[0].Reputation.Up != 0 || got[0].Reputation.Down != 1 {
		t.Errorf("ben's thumbs = %+v, want only anna's newest, one down", got[0].Reputation)
	}
}
