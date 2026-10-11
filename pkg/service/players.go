package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
)

// Players returns the player of the account followed by its guests.
func (s *Service) Players(ctx context.Context, acc account.Account) ([]player.Player, error) {
	return s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
}

// CreateGuest creates a guest player that only the account can add to its games.
func (s *Service) CreateGuest(ctx context.Context, acc account.Account, name string) (player.Player, error) {
	p := player.Player{Name: strings.TrimSpace(name), GuestOf: &acc.ID}
	if p.Name == "" {
		return p, fmt.Errorf("%w: name must not be empty", ErrInvalid)
	}
	err := s.store.StorePlayer(ctx, &p)
	return p, err
}

func (s *Service) Boards(ctx context.Context, acc account.Account) ([]board.Board, error) {
	return s.store.ListBoards(ctx, acc.ID)
}

func (s *Service) CreateBoard(ctx context.Context, acc account.Account, name string) (board.Board, error) {
	b := board.Board{AccountID: acc.ID, Name: strings.TrimSpace(name)}
	if b.Name == "" {
		return b, fmt.Errorf("%w: name must not be empty", ErrInvalid)
	}
	err := s.store.CreateBoard(ctx, &b)
	return b, err
}

func (s *Service) DeleteBoard(ctx context.Context, acc account.Account, id int64) error {
	return s.store.DeleteBoard(ctx, acc.ID, id)
}
