package service

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// resumeWithin is how far back a game's last dart may be for its turn timer to be set
// when the server starts. Older games get theirs with the next change.
const resumeWithin = 24 * time.Hour

// ServerStarted stores that the server started, so no turn ends earlier than its time after it
// (GameView.TurnDeadline; the latest start of any replica counts), and sets the turn
// timers (ResumeTurnLimits). Call it once when the server starts.
func (s *Service) ServerStarted(ctx context.Context) error {
	if err := s.opts.ServerStart.SetStarted(ctx, s.now()); err != nil {
		return err
	}
	return s.ResumeTurnLimits(ctx)
}

// ResumeTurnLimits sets the timers of the running games with a time limit for turns or a
// bot: a process has none after it started, nor after it may have missed notifications
// (Service.missed).
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

// watchGame sets the timer for the turn of a game that was changed, see watchTurn.
func (s *Service) watchGame(gameID int64) {
	g, err := s.store.GetGame(context.Background(), gameID)
	if err != nil {
		log.Printf("turn timer of game %d: %v", gameID, err)
		return
	}
	s.watchTurn(g)
}

// watchTurn sets the timer for the turn of a game that was changed, replacing the
// previous one. It fires for what comes first: a bot's next dart or takeout (botMove), or
// the end of the time for the turn (game.Settings.TurnSeconds). Both are log entries like
// a player's darts, since the server replays the log to know the state of a game.
func (s *Service) watchTurn(g game.Game) {
	if g.TurnSeconds == 0 && !g.HasBots() {
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
	delete(s.retries, g.ID) // the game changed: what failed may work now
	if g.Aborted() {
		return
	}
	at := v.TurnDeadline
	if move, ok := s.botMove(v); ok && (at == nil || move.Before(*at)) {
		at = &move
	}
	if at != nil {
		s.timers[g.ID] = time.AfterFunc(time.Until(*at), func() { s.turnTimer(g.ID) })
	}
}

// turnTimer is what happens when the timer of a game's turn fires: a bot throws or pulls
// its darts out (moveBot), or the time for the turn is up (endTurn). Under the lock it
// checks again that it is due, since the game may have moved on meanwhile, or another
// replica's timer was quicker.
func (s *Service) turnTimer(gameID int64) {
	ctx := context.Background()
	unlock, err := s.lock(ctx, "turnTimer")
	if err != nil {
		log.Printf("lock: %v", err)
		return
	}
	defer unlock()
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		log.Printf("turn timer of game %d: %v", gameID, err)
		return
	}
	v, err := s.build(ctx, g)
	if err != nil {
		log.Printf("turn timer of game %d: %v", gameID, err)
		return
	}
	if g.Aborted() {
		return
	}
	if move, ok := s.botMove(v); ok {
		if s.now().Before(move) {
			s.watchTurn(g)
			return
		}
		if err := s.moveBot(ctx, &g, v); err != nil {
			log.Printf("bot of game %d: %v", gameID, err)
			s.retryTurn(gameID)
			return
		}
		s.notifyGame(g)
		return
	}
	if err := s.endTurn(ctx, g, v); err != nil {
		log.Printf("end the turn of game %d: %v", gameID, err)
		s.retryTurn(gameID)
	}
}

// retryAfter is how long the turn timer waits to try again after it failed, e.g. the
// database couldn't be reached: nothing else sets it again while nobody changes the game.
// maxRetries is how often it tries before it gives up: an error that stays (a log that
// doesn't replay) isn't tried forever; the next change of the game sets the timer again.
const (
	retryAfter = 5 * time.Second
	maxRetries = 3
)

// retryTurn sets the game's turn timer to try again after retryAfter, up to maxRetries
// times since the game last changed.
func (s *Service) retryTurn(gameID int64) {
	s.timersMu.Lock()
	defer s.timersMu.Unlock()
	if t := s.timers[gameID]; t != nil {
		t.Stop()
		delete(s.timers, gameID)
	}
	if s.retries[gameID] >= maxRetries {
		log.Printf("turn timer of game %d: gave up after %d tries", gameID, maxRetries)
		return
	}
	s.retries[gameID]++
	s.timers[gameID] = time.AfterFunc(retryAfter, func() { s.turnTimer(gameID) })
}

// endTurn is what happens when the time for a turn is up: the turn is skipped, and after
// MaxSkips skipped turns in a row the player leaves the game (the game is aborted if
// that would leave fewer than two players, or only bots; a tournament ends for everyone,
// like when a player leaves it). While bulling off the player is out of the bull-off
// (turn.Turn.Skip); while the turn waits for the takeout the darts are taken out instead.
// The caller holds the lock (s.lock).
func (s *Service) endTurn(ctx context.Context, g game.Game, v GameView) error {
	if v.TurnDeadline == nil || v.Common().CurrentPlayerID == nil || s.now().Before(*v.TurnDeadline) {
		s.watchTurn(g) // the game moved on while the timer waited for the lock
		return nil
	}
	if v.Common().WaitingForTakeout {
		// Nobody pulled the darts out: the next player goes on, the darts count.
		if err := s.storeTakeout(ctx, &g, v); err != nil {
			return err
		}
		s.notifyGame(g)
		return nil
	}
	player := *v.Common().CurrentPlayerID
	entry := throw.Dart{GameID: g.ID, PlayerID: player}
	leaves := skippedInARow(v.Darts, player)+1 >= game.MaxSkips
	entry.Leave, entry.Skip = leaves, !leaves
	abort, err := tooFewLeft(g, append(v.Darts, entry))
	if err != nil {
		return err
	}
	if abort {
		return s.endGameOf(ctx, g)
	}
	if err := s.store.AddDart(ctx, &entry); err != nil {
		return err
	}
	s.notifyGame(g)
	return nil
}

// tooFewLeft replays the log of a game in which players leave (its last entries) and reports
// whether the game is to be aborted instead: fewer than two players would be left
// (turn.ErrTooFewLeft), or only bots (onlyBotsLeft).
func tooFewLeft(g game.Game, entries []throw.Dart) (abort bool, err error) {
	st, err := g.Play(entries)
	if errors.Is(err, turn.ErrTooFewLeft) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return onlyBotsLeft(g, st), nil
}

// endGameOf aborts a game that too few players are left in, and the tournament it is a
// match of. The caller holds the lock (s.lock).
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
