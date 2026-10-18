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
	b, err := s.store.GetBoard(ctx, boardID)
	if err != nil {
		return GameView{}, err
	}
	_, others, err := s.activeGames(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	now := time.Now()
	for _, o := range others {
		if err := s.store.SetGameBoard(ctx, o.Game.ID, nil, now); err != nil {
			return GameView{}, err
		}
		s.notifyGame(o.Game)
	}
	if err := s.store.SetGameBoard(ctx, gameID, &boardID, now); err != nil {
		return GameView{}, err
	}
	v.Game.BoardID = &boardID
	s.notifyGame(v.Game)
	return s.Game(ctx, acc, gameID)
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
