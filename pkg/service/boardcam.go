package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

// ErrNoActiveGame means a board's camera reported a dart, but no game on the board accepts darts.
var ErrNoActiveGame = errors.New("no running game on this board")

// CreateBoardToken creates a new token for the board's cameras, replacing an older one.
// The token is only returned here; the server keeps just its hash.
func (s *Service) CreateBoardToken(ctx context.Context, acc account.Account, boardID int64) (string, error) {
	b, err := s.store.GetBoard(ctx, boardID)
	if err != nil {
		return "", err
	}
	if b.AccountID != acc.ID {
		return "", ErrNotFound
	}
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if err := s.store.SetBoardToken(ctx, b.ID, hashToken(token)); err != nil {
		return "", err
	}
	return token, nil
}

// AuthenticateBoard returns the board a token belongs to.
func (s *Service) AuthenticateBoard(ctx context.Context, token string) (board.Board, error) {
	b, err := s.store.GetBoardByToken(ctx, hashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return board.Board{}, ErrUnauthenticated
	}
	return b, err
}

// AddBoardDart enters a dart detected by the board's cameras into the running game on
// the board, on behalf of the board's owner, with the position the cameras saw it at.
// While the game waits for the takeout, a new dart means the cameras missed it: the
// takeout is entered first, and the dart is the next player's (user's wish 2026-10-15;
// darts entered by hand still need the takeout).
func (s *Service) AddBoardDart(ctx context.Context, b board.Board, d throw.Dart) (GameView, error) {
	owner, v, err := s.activeGame(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	if (d.X == nil) != (d.Y == nil) {
		return GameView{}, fmt.Errorf("%w: give both x and y of the position, or neither", ErrInvalid)
	}
	if v.Common().WaitingForTakeout {
		if _, err := s.Takeout(ctx, owner, v.Game.ID); err != nil {
			return GameView{}, err
		}
	}
	return s.addDart(ctx, owner, v.Game.ID, d)
}

// BoardTakeout handles darts being pulled out of the board. If the current turn isn't
// complete then, the missing darts didn't stick (bounced off or missed the board):
// they are entered as misses, so the next player can throw.
func (s *Service) BoardTakeout(ctx context.Context, b board.Board) (GameView, error) {
	owner, v, err := s.activeGame(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	c := v.Common()
	if c.DartsInTurn == 0 {
		return v, nil
	}
	// A turn that is over (third dart, bust, checkout) needs no misses.
	missing := turn.DartsPerTurn - c.DartsInTurn
	if c.WaitingForTakeout {
		missing = 0
	}
	for range missing {
		if v, err = s.AddDart(ctx, owner, v.Game.ID, throw.Dart{Segment: throw.Miss}); err != nil {
			return GameView{}, err
		}
	}
	return s.Takeout(ctx, owner, v.Game.ID)
}

// activeGame returns the newest game on the board that accepts darts, seen by the board's owner.
func (s *Service) activeGame(ctx context.Context, b board.Board) (account.Account, GameView, error) {
	owner, games, err := s.activeGames(ctx, b)
	if err != nil {
		return account.Account{}, GameView{}, err
	}
	if len(games) == 0 {
		return account.Account{}, GameView{}, ErrNoActiveGame
	}
	return owner, games[0], nil
}

// activeGames returns the games on the board that accept darts, newest first, seen by
// the board's owner.
func (s *Service) activeGames(ctx context.Context, b board.Board) (account.Account, []GameView, error) {
	owner, err := s.store.GetAccount(ctx, b.AccountID)
	if err != nil {
		return account.Account{}, nil, err
	}
	games, err := s.store.ListGamesByBoard(ctx, b.ID, 10)
	if err != nil {
		return account.Account{}, nil, err
	}
	var active []GameView
	for _, g := range games {
		if g.Aborted() {
			continue
		}
		v, err := s.view(ctx, owner, g)
		if err != nil {
			return account.Account{}, nil, err
		}
		if v.Ready && !v.Common().GameOver {
			active = append(active, v)
		}
	}
	return owner, active, nil
}
