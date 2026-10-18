package service

import (
	"context"
	"fmt"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
)

// BoardChoice is a board a running game without a board can move to.
type BoardChoice struct {
	board.Board
	// InUse is true if another running game is on the board; it then loses the board.
	InUse bool `json:"in_use"`
}

// UseBoard puts a running game without a board on one of the account's boards, so the
// board's cameras play it. Only the owner may. A board belongs to at most one game: a
// running game that was on it continues without a board.
func (s *Service) UseBoard(ctx context.Context, acc account.Account, gameID, boardID int64) (GameView, error) {
	if err := s.checkBoard(ctx, acc, &boardID); err != nil {
		return GameView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case !v.IsOwner:
		return GameView{}, ErrForbidden
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case v.Common().GameOver:
		return GameView{}, turn.ErrGameOver
	case v.Game.BoardID != nil:
		return GameView{}, fmt.Errorf("%w: the game is on a board already", ErrInvalid)
	}
	now := time.Now()
	if err := s.freeBoard(ctx, boardID, gameID); err != nil {
		return GameView{}, err
	}
	if err := s.store.SetGameBoard(ctx, gameID, &boardID, now); err != nil {
		return GameView{}, err
	}
	v.Game.BoardID = &boardID
	s.notifyGame(v.Game)
	return s.Game(ctx, acc, gameID)
}

// freeBoard takes the board from the running games on it, except the game keep, which
// continue without a board. The caller holds s.mu.
func (s *Service) freeBoard(ctx context.Context, boardID, keep int64) error {
	b, err := s.store.GetBoard(ctx, boardID)
	if err != nil {
		return err
	}
	_, active, err := s.activeGames(ctx, b)
	if err != nil {
		return err
	}
	for _, o := range active {
		if o.Game.ID == keep {
			continue
		}
		if err := s.store.SetGameBoard(ctx, o.Game.ID, nil, time.Now()); err != nil {
			return err
		}
		o.Game.BoardID = nil
		s.notifyGame(o.Game)
	}
	return nil
}

// boardChoices lists the boards of the owner of a running game without a board.
func (s *Service) boardChoices(ctx context.Context, v GameView) ([]BoardChoice, error) {
	boards, err := s.store.ListBoards(ctx, v.Game.OwnerAccountID)
	if err != nil {
		return nil, err
	}
	choices := make([]BoardChoice, 0, len(boards))
	for _, b := range boards {
		_, active, err := s.activeGames(ctx, b)
		if err != nil {
			return nil, err
		}
		choices = append(choices, BoardChoice{Board: b, InUse: len(active) > 0})
	}
	return choices, nil
}
