package service

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// resumeWithin is how far back a game's last dart may be for its turn timer to be set
// when the server starts. Older games get theirs with the next change.
const resumeWithin = 24 * time.Hour

// ResumeTurnLimits sets the timers of the running games with a time limit for turns. The
// server has no timers of its own after it started, so call it once at the start.
func (s *Service) ResumeTurnLimits(ctx context.Context) error {
	games, err := s.store.ListTimedGames(ctx, s.now().Add(-resumeWithin))
	if err != nil {
		return err
	}
	for _, g := range games {
		s.watchTurn(g)
	}
	return nil
}

// watchTurn sets the timer for the turn of a game that was changed, replacing the
// previous one. The timer ends the turn if the player takes longer than the game's limit
// (game.Settings.TurnSeconds); it is a log entry like a dart, since the server replays
// the log to know the state of a game.
func (s *Service) watchTurn(g game.Game) {
	if g.TurnSeconds == 0 {
		return
	}
	v, err := s.build(context.Background(), g)
	if err != nil {
		log.Printf("turn timer of game %d: %v", g.ID, err)
		return
	}
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	if t := s.timers[g.ID]; t != nil {
		t.Stop()
		delete(s.timers, g.ID)
	}
	if g.Aborted() {
		return
	}
	if deadline, ok := g.TurnDeadline(v.Darts, v.Common()); ok {
		s.timers[g.ID] = time.AfterFunc(time.Until(deadline), func() { s.endTurn(g.ID) })
	}
}

// endTurn is what happens when the time for a turn is up: the turn is skipped, and after
// MaxSkips skipped turns in a row the player leaves the game (the game is aborted if
// that would leave fewer than two players; a tournament ends for everyone, like when a
// player leaves it).
func (s *Service) endTurn(gameID int64) {
	ctx := context.Background()
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		log.Printf("end the turn of game %d: %v", gameID, err)
		return
	}
	v, err := s.build(ctx, g)
	if err != nil {
		log.Printf("end the turn of game %d: %v", gameID, err)
		return
	}
	deadline, ok := g.TurnDeadline(v.Darts, v.Common())
	if g.Aborted() || !ok || v.Common().CurrentPlayerID == nil || s.now().Before(deadline) {
		s.watchTurn(g) // the game moved on while the timer waited for the lock
		return
	}
	player := *v.Common().CurrentPlayerID
	entry := throw.Dart{GameID: gameID, PlayerID: player}
	leaves := skippedInARow(v.Darts, player)+1 >= game.MaxSkips
	if leaves {
		active := 0
		for _, left := range v.Common().Records {
			if !left.Left {
				active++
			}
		}
		if active <= 2 {
			err = s.endGameOf(ctx, g)
			if err != nil {
				log.Printf("end game %d: %v", gameID, err)
			}
			return
		}
	}
	entry.Leave, entry.Skip = leaves, !leaves
	if _, err = g.Play(append(v.Darts, entry)); err == nil {
		err = s.store.AddDart(ctx, &entry)
	}
	if err != nil {
		log.Printf("end the turn of game %d: %v", gameID, err)
		return
	}
	s.notifyGame(g)
}

// endGameOf aborts a game that too few players are left in, and the tournament it is a
// match of. The caller holds s.mu.
func (s *Service) endGameOf(ctx context.Context, g game.Game) error {
	if g.TournamentID == nil {
		return s.abortGame(ctx, &g)
	}
	t, err := s.store.GetTournament(ctx, *g.TournamentID)
	if err != nil {
		return err
	}
	return s.abortTournament(ctx, &t)
}

// skippedInARow counts the turns the player skipped since they last threw a dart.
func skippedInARow(entries []throw.Dart, playerID int64) int {
	n := 0
	for _, e := range slices.Backward(entries) {
		switch {
		case e.PlayerID != playerID:
		case e.Skip:
			n++
		case e.IsDart():
			return n
		}
	}
	return n
}
