package service

import (
	"context"
	"errors"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"strconv"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

func TestDeleteGuest(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	oma, _ := s.CreateGuest(ctx, anna, "Oma")

	// A finished game of anna and papa, and a running one with oma.
	quick := func(ids ...int64) game.Game {
		g := x01Game(ids...)
		g.Rules.(*x01.Settings).StartScore = 40
		return g
	}
	done, err := s.CreateGame(ctx, anna, quick(anna.PlayerID, papa.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDart(ctx, anna, done.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	running, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID, oma.ID), nil)

	if err := s.DeleteGuest(ctx, ben, papa.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben deletes anna's guest: got %v, want ErrNotFound", err)
	}
	if err := s.DeleteGuest(ctx, anna, anna.PlayerID); !errors.Is(err, ErrNotFound) {
		t.Errorf("anna deletes her own player as a guest: got %v, want ErrNotFound", err)
	}
	if err := s.DeleteGuest(ctx, anna, oma.ID); !errors.Is(err, ErrGuestBusy) {
		t.Errorf("delete a guest in a running game: got %v, want ErrGuestBusy", err)
	}

	before, _ := s.Stats(ctx, anna, "")
	if err := s.DeleteGuest(ctx, anna, papa.ID); err != nil {
		t.Fatal(err)
	}
	players, _ := s.Players(ctx, anna)
	if len(players) != 2 { // anna and oma
		t.Errorf("anna's players = %+v, want herself and oma", players)
	}
	v, err := s.Game(ctx, anna, done.Game.ID)
	if err != nil || len(v.Players) != 2 {
		t.Fatalf("the finished game: %v, %d players", err, len(v.Players))
	}
	for _, p := range v.Players {
		if p.ID == papa.ID && (!p.Deleted() || p.Name != "" || p.IsGuest()) {
			t.Errorf("papa after deleting = %+v, want deleted, nameless, nobody's guest", p)
		}
	}
	if !v.Common().GameOver || v.Common().WinnerID == nil || *v.Common().WinnerID != anna.PlayerID {
		t.Error("the finished game changed")
	}
	after, _ := s.Stats(ctx, anna, "")
	if before[0].X01 != after[0].X01 {
		t.Errorf("anna's statistics changed: %+v → %+v", before[0].X01, after[0].X01)
	}

	// Once the running game is over, oma can go, too.
	if _, err := s.AddDart(ctx, anna, running.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteGuest(ctx, anna, oma.ID); err != nil {
		t.Errorf("delete a guest after the game: %v", err)
	}
}

func TestDeleteAccount(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara, dora := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara"), register(t, s, "dora")
	befriend(t, s, anna, ben)
	befriend(t, s, ben, cara)
	befriend(t, s, anna, cara)
	befriend(t, s, ben, dora)
	keller, _ := s.CreateBoard(ctx, ben, "Keller")
	bensGuest, _ := s.CreateGuest(ctx, ben, "Bens Gast")
	// A player is in one game or tournament at a time: those below are each of other players.
	bensOthers := make([]int64, 2)
	for i := range bensOthers {
		g, _ := s.CreateGuest(ctx, ben, "Bens Gast "+strconv.Itoa(i+2))
		bensOthers[i] = g.ID
	}
	annasGuest, _ := s.CreateGuest(ctx, anna, "Annas Gast")
	carasGuest, _ := s.CreateGuest(ctx, cara, "Caras Gast")

	quick := func(ids ...int64) game.Game {
		g := x01Game(ids...)
		g.Rules.(*x01.Settings).StartScore = 40
		return g
	}
	// A finished game of ben and anna, kept as it was.
	done, _ := s.CreateGame(ctx, anna, quick(anna.PlayerID), []string{"ben"})
	_, _ = s.AcceptInvitation(ctx, ben, done.Game.ID, nil)
	_, _ = s.AddDart(ctx, anna, done.Game.ID, d20)
	// A running game of anna's guest with ben and cara: ben leaves it, it goes on.
	three, _ := s.CreateGame(ctx, anna, x01Game(annasGuest.ID), []string{"ben", "cara"})
	_, _ = s.AcceptInvitation(ctx, ben, three.Game.ID, nil)
	_, _ = s.AcceptInvitation(ctx, cara, three.Game.ID, nil)
	// A running game of ben's guest, with anna: it is aborted.
	bens, _ := s.CreateGame(ctx, ben, x01Game(bensGuest.ID), []string{"anna"})
	_, _ = s.AcceptInvitation(ctx, anna, bens.Game.ID, nil)
	// An invitation of ben's that nobody answered: declined.
	invited, _ := s.CreateGame(ctx, cara, x01Game(carasGuest.ID), []string{"ben"})
	// A tournament of ben's guests with dora: aborted.
	tv, _ := s.CreateTournament(ctx, ben, quickTournament(tournament.RoundRobin, bensOthers...), []string{"dora"})
	_, _ = s.AcceptTournamentInvitation(ctx, dora, tv.Tournament.ID, nil)

	if err := s.DeleteAccount(ctx, ben, "wrong password"); !errors.Is(err, ErrWrongPassword) {
		t.Fatalf("wrong password: got %v, want ErrWrongPassword", err)
	}
	before, _ := s.Stats(ctx, anna, "")
	if err := s.DeleteAccount(ctx, ben, "password"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Login(ctx, "ben", "password"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("login after deleting: got %v, want ErrUnauthenticated", err)
	}
	if _, err := s.Register(ctx, "ben", "", "", "password"); err != nil {
		t.Errorf("the username is free again: %v", err)
	}
	if fr, _ := s.Friends(ctx, anna); len(fr.Friends) != 1 || fr.Friends[0].Username != "cara" {
		t.Errorf("anna's friends = %+v, want only cara", fr.Friends)
	}
	if _, err := s.store.GetBoard(ctx, keller.ID); err == nil {
		t.Error("ben's board is still there")
	}

	if v, _ := s.Game(ctx, anna, done.Game.ID); !v.Common().GameOver || len(v.Players) != 2 || !playerDeleted(v, ben.PlayerID) {
		t.Errorf("the finished game: over %v, ben deleted %v; want it kept with ben as a deleted player", v.Common().GameOver, playerDeleted(v, ben.PlayerID))
	}
	if v, _ := s.Game(ctx, anna, three.Game.ID); v.Game.Aborted() || !v.Common().Records[ben.PlayerID].Left {
		t.Errorf("the game of three: aborted %v, ben left %v; want it to go on without ben", v.Game.Aborted(), v.Common().Records[ben.PlayerID].Left)
	}
	if v, _ := s.Game(ctx, anna, bens.Game.ID); !v.Game.Aborted() {
		t.Error("ben's own running game wasn't aborted")
	}
	if v, _ := s.Game(ctx, cara, invited.Game.ID); len(v.Game.Participants) != 1 {
		t.Errorf("the open invitation: players %v; want it withdrawn", v.Game.PlayerIDs())
	}
	if v, _ := s.Tournament(ctx, dora, tv.Tournament.ID); !v.Tournament.Aborted() {
		t.Error("the tournament ben took part in wasn't aborted")
	}
	after, _ := s.Stats(ctx, anna, "")
	if before[0].X01 != after[0].X01 {
		t.Errorf("anna's statistics changed: %+v → %+v", before[0].X01, after[0].X01)
	}
}

func playerDeleted(v GameView, id int64) bool {
	for _, p := range v.Players {
		if p.ID == id {
			return p.Deleted() && p.Name == ""
		}
	}
	return false
}
