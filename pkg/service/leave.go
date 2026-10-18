package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// ErrCantLeave means the game can't be left now, e.g. while the players bull off.
var ErrCantLeave = turn.ErrCantLeave

// LeaveGame lets a player with an account leave a game that isn't theirs (the host ends
// a game by aborting it; user's decision 2026-10-18):
//
//   - In the lobby the player is simply removed, which frees the seat of an open game.
//     The returned view is empty then, the game isn't theirs anymore.
//   - In a running game the others play on without them (a log entry throw.Dart.Leave,
//     the legs and sets they won stay theirs), unless fewer than two players would be
//     left: then the game is aborted.
//
// Games of a tournament can't be left, and a player can't leave while bulling off.
func (s *Service) LeaveGame(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case v.IsOwner, v.Invited, v.Game.TournamentID != nil:
		return GameView{}, ErrForbidden
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case v.Common().GameOver:
		return GameView{}, turn.ErrGameOver
	case v.HasLeft:
		return GameView{}, fmt.Errorf("%w: you left the game already", ErrInvalid)
	}

	return s.leave(ctx, acc, v)
}

// leave is LeaveGame after the checks. The caller holds s.mu.
func (s *Service) leave(ctx context.Context, acc account.Account, v GameView) (GameView, error) {
	gameID := v.Game.ID
	if !v.Ready { // still in the lobby
		if err := s.store.RemoveInvitedPlayer(ctx, gameID, acc.PlayerID); err != nil {
			return GameView{}, err
		}
		s.notifyGame(v.Game)
		return GameView{}, nil
	}

	active := 0
	for _, id := range v.Game.PlayerIDs {
		if !v.Common().Records[id].Left {
			active++
		}
	}
	if active <= 2 {
		if err := s.abortGame(ctx, &v.Game); err != nil {
			return GameView{}, err
		}
		return s.view(ctx, acc, v.Game)
	}
	entry := throw.Dart{GameID: gameID, PlayerID: acc.PlayerID, Leave: true}
	if _, err := v.Game.Play(append(v.Darts, entry)); err != nil {
		if errors.Is(err, turn.ErrCantLeave) {
			return GameView{}, err
		}
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.AddDart(ctx, &entry); err != nil {
		return GameView{}, err
	}
	s.notifyGame(v.Game)
	return s.view(ctx, acc, v.Game)
}
