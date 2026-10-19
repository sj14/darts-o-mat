package service

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

// BoardChoice is a board a player can throw on.
type BoardChoice struct {
	board.Board
	// InUse is true if another running game is on the board; it then loses the board.
	InUse bool `json:"in_use"`
	// Others is true for a board of another account that other players of the game throw
	// on: friends at one board share it.
	Others bool `json:"others,omitempty"`
}

// UseBoard puts the account's players of a running game on a board, so the board's
// cameras report their darts: one of the account's boards, or one the other players of
// the game throw on. A board belongs to at most one game: a running game that was on it
// continues without it. With a nil board the players are taken off theirs, e.g. because
// it broke: from then on they enter their darts by hand, as if they had played without a
// board from the start (the game stops waiting for takeouts if no one else has a board).
func (s *Service) UseBoard(ctx context.Context, acc account.Account, gameID int64, boardID *int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}
	if err := s.checkGameBoard(ctx, acc, g, boardID); err != nil {
		return GameView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.view(ctx, acc, g)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case v.Common().GameOver:
		return GameView{}, turn.ErrGameOver
	case !v.CanThrow:
		return GameView{}, ErrForbidden
	}
	now := time.Now()
	for _, id := range v.Mine {
		if v.Common().Records[id].Left {
			continue
		}
		if err := s.store.SetParticipantBoard(ctx, gameID, id, boardID, now); err != nil {
			return GameView{}, err
		}
	}
	if err := s.takeBoard(ctx, boardID, gameID); err != nil {
		return GameView{}, err
	}
	if g, err = s.store.GetGame(ctx, gameID); err != nil {
		return GameView{}, err
	}
	s.notifyGame(g)
	return s.view(ctx, acc, g)
}

// freeBoard takes the board from the players of the running games on it, except the game
// keep, which continue without it. The caller holds s.mu.
func (s *Service) freeBoard(ctx context.Context, boardID, keep int64) error {
	b, err := s.store.GetBoard(ctx, boardID)
	if err != nil {
		return err
	}
	active, err := s.activeGames(ctx, b)
	if err != nil {
		return err
	}
	for _, o := range active {
		if o.Game.ID == keep {
			continue
		}
		if err := s.store.ReleaseBoard(ctx, o.Game.ID, boardID); err != nil {
			return err
		}
		s.notifyGame(o.Game)
	}
	return nil
}

// takeBoard is freeBoard for a board that may be nil.
func (s *Service) takeBoard(ctx context.Context, boardID *int64, keep int64) error {
	if boardID == nil {
		return nil
	}
	return s.freeBoard(ctx, *boardID, keep)
}

// takeBoards frees the boards the players of a new game throw on from other games.
func (s *Service) takeBoards(ctx context.Context, g game.Game) error {
	var done []int64
	for _, p := range g.Participants {
		if p.BoardID == nil || slices.Contains(done, *p.BoardID) {
			continue
		}
		done = append(done, *p.BoardID)
		if err := s.freeBoard(ctx, *p.BoardID, g.ID); err != nil {
			return err
		}
	}
	return nil
}

// boardChoices lists the boards an account can pick in a game: its own and those other
// players of the game throw on.
func (s *Service) boardChoices(ctx context.Context, acc account.Account, g game.Game) ([]BoardChoice, error) {
	boards, err := s.store.ListBoards(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	choices := make([]BoardChoice, 0, len(boards))
	for _, b := range boards {
		active, err := s.activeGames(ctx, b)
		if err != nil {
			return nil, err
		}
		inUse := slices.ContainsFunc(active, func(o GameView) bool { return o.Game.ID != g.ID })
		choices = append(choices, BoardChoice{Board: b, InUse: inUse})
	}
	for _, p := range g.Participants {
		if p.BoardID == nil || slices.ContainsFunc(choices, func(c BoardChoice) bool { return c.ID == *p.BoardID }) {
			continue
		}
		b, err := s.store.GetBoard(ctx, *p.BoardID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		choices = append(choices, BoardChoice{Board: b, Others: true})
	}
	return choices, nil
}

// tournamentBoardChoices lists the boards an account can pick in a tournament: its own and
// those other players of the tournament throw on.
func (s *Service) tournamentBoardChoices(ctx context.Context, acc account.Account, t tournament.Tournament) ([]BoardChoice, error) {
	boards, err := s.store.ListBoards(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	choices := make([]BoardChoice, 0, len(boards))
	for _, b := range boards {
		choices = append(choices, BoardChoice{Board: b})
	}
	for _, p := range t.Participants {
		if p.BoardID == nil || slices.ContainsFunc(choices, func(c BoardChoice) bool { return c.ID == *p.BoardID }) {
			continue
		}
		b, err := s.store.GetBoard(ctx, *p.BoardID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		choices = append(choices, BoardChoice{Board: b, Others: true})
	}
	return choices, nil
}
