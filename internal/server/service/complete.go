package service

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
)

// CompleteGame confirms the result of a finished game: afterwards its darts can't be
// undone or corrected anymore, and in a tournament the winner goes on. Only the player
// who threw the last dart may (user's decision 2026-10-20: the cameras may have got that
// dart wrong, and it is not necessarily the winner's in a future mode); the others, whoever
// left and whoever has only an invitation get ErrForbidden. Completing a completed game
// changes nothing; a game that isn't over is ErrNotOver. If nobody does, it confirms
// itself after game.CompleteAfter.
func (s *Service) CompleteGame(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	switch {
	case err != nil:
		return GameView{}, err
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case v.Invited || v.HasLeft:
		return GameView{}, ErrForbidden
	case !v.Common().GameOver:
		return GameView{}, ErrNotOver
	case v.Completed:
		return v, nil
	case !v.MayComplete():
		return GameView{}, ErrForbidden
	}
	if err := s.completeGame(ctx, &v.Game); err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, v.Game)
}

// completeGame stores that the game is completed and tells its subscribers. The caller
// holds s.mu.
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
// confirmed, because one of its players starts another game: whoever starts the next game
// has seen the result. Only those whose last dart one of its players threw (MayComplete),
// as it is theirs to confirm. Only games with a dart in the last game.CompleteAfter can be
// affected, older finished games are completed by the time anyway. The games that are
// still running aren't touched, so an account can play on several boards at once. The
// caller holds s.mu.
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
		if v.MayComplete() {
			if err := s.completeGame(ctx, &g); err != nil {
				return err
			}
		}
	}
	return nil
}
