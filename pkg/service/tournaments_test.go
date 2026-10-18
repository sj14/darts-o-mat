package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
)

// quickTournament has games that the first dart, a D20, wins.
func quickTournament(format tournament.Format, playerIDs ...int64) tournament.Tournament {
	return tournament.Tournament{
		Format:    format,
		PlayerIDs: playerIDs,
		Settings:  game.Settings{Mode: game.ModeX01, X01: &x01.Settings{StartScore: 40, OutMode: x01.OutModeSingle}},
	}
}

var d20 = throw.Dart{Number: 20, Segment: throw.Double}

func TestRoundRobinTournament(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	g1, err := s.CreateGuest(ctx, anna, "Gast 1")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := s.CreateGuest(ctx, anna, "Gast 2")
	if err != nil {
		t.Fatal(err)
	}

	v, err := s.CreateTournament(ctx, anna, quickTournament(tournament.RoundRobin, anna.PlayerID, g1.ID, g2.ID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Tournament.ID
	if v.Ready || len(v.State.Matches) != 6 {
		t.Fatalf("ready %v with %d matches, want waiting for ben with 6 matches", v.Ready, len(v.State.Matches))
	}
	if _, err := s.StartMatch(ctx, anna, id, 0); !errors.Is(err, ErrNotReady) {
		t.Fatalf("start before ben accepted: %v, want ErrNotReady", err)
	}
	if inv, err := s.TournamentInvitations(ctx, ben); err != nil || len(inv) != 1 {
		t.Fatalf("ben's invitations: %v, %v", inv, err)
	}
	if v, err = s.AcceptTournamentInvitation(ctx, ben, id, nil); err != nil || !v.Ready || !v.CanPlay {
		t.Fatalf("accept: ready %v can play %v, %v", v.Ready, v.CanPlay, err)
	}

	for _, m := range v.State.Matches {
		// Ben may only start his own matches.
		mine := m.PlayerIDs[0] == ben.PlayerID || m.PlayerIDs[1] == ben.PlayerID
		if _, err := s.StartMatch(ctx, ben, id, m.Index); !mine && !errors.Is(err, ErrForbidden) {
			t.Errorf("ben starts match %v: %v, want ErrForbidden", m.PlayerIDs, err)
		} else if mine {
			if err != nil {
				t.Fatalf("ben starts his match %v: %v", m.PlayerIDs, err)
			}
			continue
		}
		if _, err := s.StartMatch(ctx, anna, id, m.Index); err != nil {
			t.Fatalf("start match %d: %v", m.Index, err)
		}
	}
	if _, err := s.StartMatch(ctx, anna, id, 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("starting a running match again: %v, want ErrInvalid", err)
	}

	// The first player of each match wins with one dart.
	v, err = s.Tournament(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range v.State.Matches {
		// Ben enters his own darts, anna those of hers and her guests.
		actor := anna
		if gv, _ := s.Game(ctx, anna, m.Result.GameID); *gv.Common().CurrentPlayerID == ben.PlayerID {
			actor = ben
		}
		if _, err := s.AddDart(ctx, actor, m.Result.GameID, d20); err != nil {
			t.Fatalf("match %d: %v", m.Index, err)
		}
	}
	v, err = s.Tournament(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	if !v.State.Over || v.State.WinnerID == nil || *v.State.WinnerID != v.State.Standings[0].PlayerID {
		t.Fatalf("over %v winner %v, want over with the leader of the table", v.State.Over, v.State.WinnerID)
	}
	played := 0
	for _, s := range v.Stats {
		played += s.X01.Games
	}
	if played != 12 {
		t.Errorf("tournament statistics count %d player games, want 6 matches × 2", played)
	}

	// Each of anna's players took part in one finished tournament.
	summaries, err := s.Stats(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	wins := 0
	for _, sum := range summaries {
		if sum.X01.Tournaments != 1 {
			t.Errorf("%s: %d tournaments, want 1", sum.Player.Name, sum.X01.Tournaments)
		}
		wins += sum.X01.TournamentWins
	}
	if winnerIsBen := *v.State.WinnerID == ben.PlayerID; wins != 1 && !winnerIsBen || wins != 0 && winnerIsBen {
		t.Errorf("anna's players won %d tournaments, winner %d", wins, *v.State.WinnerID)
	}
}

func TestKnockoutTournament(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	ids := []int64{anna.PlayerID}
	for _, name := range []string{"B", "C"} {
		g, err := s.CreateGuest(ctx, anna, name)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, g.ID)
	}
	v, err := s.CreateTournament(ctx, anna, quickTournament(tournament.Knockout, ids...), nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Tournament.ID
	// 3 players: seed 1 has a bye, 2 and 3 play, then the final.
	if _, err := s.StartMatch(ctx, anna, id, 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("starting a bye: %v, want ErrInvalid", err)
	}
	if _, err := s.StartMatch(ctx, anna, id, 2); !errors.Is(err, ErrInvalid) {
		t.Errorf("starting the final before its players are known: %v, want ErrInvalid", err)
	}
	semi, err := s.StartMatch(ctx, anna, id, 1)
	if err != nil {
		t.Fatal(err)
	}
	// An aborted game doesn't decide the match; it can be started again.
	if _, err := s.AbortGame(ctx, anna, semi.Game.ID); err != nil {
		t.Fatal(err)
	}
	if semi, err = s.StartMatch(ctx, anna, id, 1); err != nil {
		t.Fatalf("restart after abort: %v", err)
	}
	if _, err := s.AddDart(ctx, anna, semi.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	final, err := s.StartMatch(ctx, anna, id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if final.Game.PlayerIDs[0] != v.Tournament.PlayerIDs[0] || final.Game.PlayerIDs[1] != semi.Game.PlayerIDs[0] {
		t.Errorf("final between %v, want seed 1 and the winner of the semi-final", final.Game.PlayerIDs)
	}
	if _, err := s.AddDart(ctx, anna, final.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	v, err = s.Tournament(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	if !v.State.Over || *v.State.WinnerID != v.Tournament.PlayerIDs[0] {
		t.Errorf("over %v winner %v, want seed 1", v.State.Over, v.State.WinnerID)
	}
	// The database records when each dart was thrown; the tournament lasts from the
	// first dart of its games to the last.
	if tm := v.Time; tm.Start.IsZero() || tm.End.Before(tm.Start) {
		t.Errorf("tournament played %v, want the times of its darts", tm)
	}
	if _, err := s.AbortTournament(ctx, anna, id); !errors.Is(err, ErrInvalid) {
		t.Errorf("aborting a finished tournament: %v, want ErrInvalid", err)
	}
}

func TestSubscribeTournament(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	ids := []int64{anna.PlayerID}
	for _, name := range []string{"B", "C"} {
		g, err := s.CreateGuest(ctx, anna, name)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, g.ID)
	}
	v, err := s.CreateTournament(ctx, anna, quickTournament(tournament.RoundRobin, ids...), nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.CreateTournament(ctx, anna, quickTournament(tournament.RoundRobin, ids...), nil)
	if err != nil {
		t.Fatal(err)
	}

	changes, stop := s.SubscribeTournament(v.Tournament.ID)
	defer stop()
	pending := func() bool {
		select {
		case <-changes:
			return true
		default:
			return false
		}
	}

	otherGame, err := s.StartMatch(ctx, anna, other.Tournament.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDart(ctx, anna, otherGame.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	if pending() {
		t.Error("notified about another tournament")
	}

	game, err := s.StartMatch(ctx, anna, v.Tournament.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !pending() {
		t.Error("no notification after starting a match")
	}
	if _, err := s.AddDart(ctx, anna, game.Game.ID, d20); err != nil {
		t.Fatal(err)
	}
	if !pending() {
		t.Error("no notification after a dart of one of its games")
	}
}
