package service

import (
	"context"
	"fmt"
	"slices"
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
	existing, err := s.Players(ctx, acc)
	if err != nil {
		return p, err
	}
	if slices.ContainsFunc(existing, func(e player.Player) bool { return strings.EqualFold(e.Name, p.Name) }) {
		return p, ErrGuestNameTaken
	}
	err = s.store.StorePlayer(ctx, &p)
	return p, err
}

// BoardView is a board with whether its cameras can report darts.
type BoardView struct {
	board.Board
	HasToken bool `json:"has_token"`
}

func (s *Service) Boards(ctx context.Context, acc account.Account) ([]BoardView, error) {
	boards, err := s.store.ListBoards(ctx, acc.ID)
	if err != nil {
		return nil, err
	}
	views := []BoardView{}
	for _, b := range boards {
		has, err := s.store.HasBoardToken(ctx, b.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, BoardView{Board: b, HasToken: has})
	}
	return views, nil
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
