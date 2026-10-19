package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
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

// Player returns the account's own player.
func (s *Service) Player(ctx context.Context, acc account.Account) (player.Player, error) {
	players, err := s.store.GetPlayers(ctx, []int64{acc.PlayerID})
	if err != nil || len(players) == 0 {
		return player.Player{}, err
	}
	return players[0], nil
}

// SetColor sets the accent color of the account's own player; empty means the default
// from its name. Guests can't have their own.
func (s *Service) SetColor(ctx context.Context, acc account.Account, color string) error {
	if color != "" && !slices.Contains(player.Colors, color) {
		return fmt.Errorf("%w: unknown color %q", ErrInvalid, color)
	}
	return s.store.SetPlayerColor(ctx, acc.PlayerID, color)
}

// BoardView is a board with whether its cameras can report darts.
type BoardView struct {
	board.Board
	HasToken bool `json:"has_token"`
	// InUse is true if a running game is on the board; a new game on it takes it over.
	InUse bool `json:"in_use"`
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
		active, err := s.activeGames(ctx, b)
		if err != nil {
			return nil, err
		}
		views = append(views, BoardView{Board: b, HasToken: has, InUse: len(active) > 0})
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
