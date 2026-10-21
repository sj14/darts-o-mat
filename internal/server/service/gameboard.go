package service

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store"
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
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
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
	if err := s.checkSeatBoard(ctx, acc, gameSeat(v.Game), boardID); err != nil {
		return GameView{}, err
	}
	now := s.now()
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
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}
	s.notifyGame(g)
	return s.view(ctx, acc, g)
}

// ErrBoardBusy means the board is in a running game that is only for players with a
// board: it can't be taken from it.
var ErrBoardBusy = fmt.Errorf("%w: the board is in a game that needs it", ErrInvalid)

// ErrBoardRequired means the game or tournament is only for players with a board, and the
// player has none.
var ErrBoardRequired = fmt.Errorf("%w: a board is required", ErrInvalid)

// seat is the game or tournament a player takes a seat in, as checkSeatBoard sees it.
type seat struct {
	lobby game.Lobby
	// shared are the boards the other players throw on: friends at one board share it.
	shared []int64
	// game is the game that takes the board, 0 for one that isn't stored yet. A tournament
	// takes no board (holdsBoards false), only the games of its matches do.
	game        int64
	holdsBoards bool
}

// gameSeat is a seat in a stored game.
func gameSeat(g game.Game) seat {
	st := seat{lobby: g.Lobby, game: g.ID, holdsBoards: true}
	for _, p := range g.Participants {
		if p.BoardID != nil {
			st.shared = append(st.shared, *p.BoardID)
		}
	}
	return st
}

// tournamentSeat is a seat in a stored tournament.
func tournamentSeat(t tournament.Tournament) seat {
	st := seat{lobby: t.Lobby}
	for _, p := range t.Participants {
		if p.BoardID != nil {
			st.shared = append(st.shared, *p.BoardID)
		}
	}
	return st
}

// checkSeatBoard checks the board a player takes a seat on, nil for none: a lobby only
// for players with a board needs one (ErrBoardRequired); it is one the other players
// throw on or one of the account's; and a game can't take it from a running game that
// needs its boards (ErrBoardBusy).
func (s *Service) checkSeatBoard(ctx context.Context, acc account.Account, st seat, boardID *int64) error {
	if boardID == nil {
		if st.lobby.BoardOnly {
			return ErrBoardRequired
		}
		return nil
	}
	if slices.Contains(st.shared, *boardID) {
		return nil
	}
	b, err := s.store.GetBoard(ctx, *boardID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && b.AccountID != acc.ID) {
		return fmt.Errorf("%w: unknown board", ErrInvalid)
	}
	if err != nil || !st.holdsBoards {
		return err
	}
	return s.checkBoardFree(ctx, *boardID, st.game)
}

// checkBoardFree checks that taking the board for the game keep (0 for a game that isn't
// stored yet) wouldn't take it from a running game that needs its boards (game.Lobby.BoardOnly).
func (s *Service) checkBoardFree(ctx context.Context, boardID, keep int64) error {
	b, err := s.store.GetBoard(ctx, boardID)
	if err != nil {
		return err
	}
	active, err := s.activeGames(ctx, b)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(active, func(o GameView) bool { return o.Game.ID != keep && o.Game.BoardOnly }) {
		return ErrBoardBusy
	}
	return nil
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

// boardChoices lists the boards an account can pick in a game: its own, marked if another
// running game is on them, and those other players of the game throw on.
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
	return s.withSharedBoards(ctx, choices, gameSeat(g).shared)
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
	return s.withSharedBoards(ctx, choices, tournamentSeat(t).shared)
}

// withSharedBoards adds the boards other players throw on (seat.shared) to the account's
// choices, unless they are among them already.
func (s *Service) withSharedBoards(ctx context.Context, choices []BoardChoice, shared []int64) ([]BoardChoice, error) {
	for _, id := range shared {
		if slices.ContainsFunc(choices, func(c BoardChoice) bool { return c.ID == id }) {
			continue
		}
		b, err := s.store.GetBoard(ctx, id)
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
