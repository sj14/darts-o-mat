package service

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
)

// completeGame stores that the game is completed: afterwards its darts can't be undone or
// corrected anymore, and its result counts. It tells its subscribers. The caller holds
// the lock (s.lock).
func (s *Service) completeGame(ctx context.Context, g *game.Game) error {
	now := s.now()
	g.CompletedAt = &now
	if err := s.store.CompleteGame(ctx, g); err != nil {
		return err
	}
	s.notifyGame(*g)
	return nil
}

// completeFinished completes the games of the account that just finished and weren't
// completed, because one of its players goes on to another game or tournament: whoever
// goes on accepts the result (user's decision 2026-10-20, so that the result, and a rating
// from it, is final before the next game; there is no button to confirm it). Not those the
// account left or is only invited to. Only games with a dart in the last
// game.CompleteAfter can be affected, older finished games are completed by the time
// anyway. The games that are still running aren't touched, so an account can play on
// several boards at once. The caller holds the lock (s.lock).
func (s *Service) completeFinished(ctx context.Context, acc account.Account) error {
	games, err := s.store.ListGamesWithDartsSince(ctx, acc.ID, acc.PlayerID, s.now().Add(-game.CompleteAfter-time.Minute))
	if err != nil {
		return err
	}
	for _, g := range games {
		v, err := s.view(ctx, acc, g)
		if err != nil {
			return err
		}
		if v.Common().GameOver && !v.Completed && !v.Invited && !v.HasLeft {
			if err := s.completeGame(ctx, &g); err != nil {
				return err
			}
		}
	}
	return nil
}
