package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

var (
	// ErrNoActiveGame means a board's camera reported a dart, but no game on the board accepts darts.
	ErrNoActiveGame = errors.New("no running game on this board")
	// ErrNotOnBoard means a board reported a dart, but the player who is up throws on another one.
	ErrNotOnBoard = errors.New("the player who is up doesn't throw on this board")
)

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
// the board, with the position the cameras saw it at. While the game waits for the
// takeout, a new dart means the cameras missed it: the takeout is entered first, and the
// dart is the next player's (user's wish 2026-10-15; darts entered by hand still need
// the takeout). A board only counts the darts of the turns of players who throw on it: a
// dart of another board is refused (ErrNotOnBoard).
func (s *Service) AddBoardDart(ctx context.Context, b board.Board, d throw.Dart) (GameView, error) {
	v, err := s.activeGame(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	if (d.X == nil) != (d.Y == nil) {
		return GameView{}, fmt.Errorf("%w: give both x and y of the position, or neither", ErrInvalid)
	}
	return s.changeByCameras(ctx, v.Game.ID, func(g *game.Game, v GameView) error {
		if v.Common().WaitingForTakeout {
			if err := s.storeTakeout(ctx, g, v); err != nil {
				return err
			}
			var err error
			if v, err = s.build(ctx, *g); err != nil {
				return err
			}
		}
		if !v.PlaysOn(b.ID) {
			return ErrNotOnBoard
		}
		return s.storeDart(ctx, g, v, d)
	})
}

// BoardTakeout handles darts being pulled out of the board. If the current turn isn't
// complete then, the missing darts didn't stick (bounced off or missed the board):
// they are entered as misses, so the next player can throw.
func (s *Service) BoardTakeout(ctx context.Context, b board.Board) (GameView, error) {
	v, err := s.activeGame(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	return s.changeByCameras(ctx, v.Game.ID, func(g *game.Game, v GameView) error {
		c := v.Common()
		if c.DartsInTurn == 0 || !v.PlaysOn(b.ID) {
			return nil
		}
		// A turn that is over (third dart, bust, checkout) needs no misses.
		missing := turn.DartsPerTurn - c.DartsInTurn
		if c.WaitingForTakeout {
			missing = 0
		}
		for range missing {
			if err := s.storeDart(ctx, g, v, throw.Dart{Segment: dartboard.Miss}); err != nil {
				return err
			}
			var err error
			if v, err = s.build(ctx, *g); err != nil {
				return err
			}
		}
		return s.storeTakeout(ctx, g, v)
	})
}

// activeGame returns the newest game with a player on the board that accepts darts.
func (s *Service) activeGame(ctx context.Context, b board.Board) (GameView, error) {
	games, err := s.activeGames(ctx, b)
	if err != nil {
		return GameView{}, err
	}
	if len(games) == 0 {
		return GameView{}, ErrNoActiveGame
	}
	return games[0], nil
}

// activeGames returns the games with a player on the board that accept darts, newest
// first.
func (s *Service) activeGames(ctx context.Context, b board.Board) ([]GameView, error) {
	games, err := s.store.ListGamesByBoard(ctx, b.ID, 10)
	if err != nil {
		return nil, err
	}
	var active []GameView
	for _, g := range games {
		if g.Aborted() {
			continue
		}
		v, err := s.build(ctx, g)
		if err != nil {
			return nil, err
		}
		if v.Ready && !v.Common().GameOver {
			active = append(active, v)
		}
	}
	return active, nil
}
