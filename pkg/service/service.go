// Package service contains the use cases shared by the JSON API and the web frontend.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

var (
	// ErrInvalid wraps errors caused by invalid input, e.g. unknown players or wrong settings.
	ErrInvalid       = errors.New("invalid input")
	ErrNotFound      = store.ErrNotFound
	ErrNothingToUndo = errors.New("no dart to undo")
)

type Service struct {
	store *store.Store
	// dartsMu serializes changes to the darts, so two darts can't both pass
	// the replay check against the same previous state.
	dartsMu sync.Mutex
}

func New(store *store.Store) *Service {
	return &Service{store: store}
}

// GameView is a game together with everything needed to show it.
type GameView struct {
	Game    game.Game       `json:"game"`
	Players []player.Player `json:"players"` // in throwing order
	Darts   []throw.Dart    `json:"darts"`
	State   any             `json:"state"` // x01.State or aroundtheclock.State, depending on the mode
}

func (s *Service) ListPlayers(ctx context.Context) ([]player.Player, error) {
	return s.store.ListPlayers(ctx)
}

func (s *Service) CreatePlayer(ctx context.Context, name string) (player.Player, error) {
	p := player.Player{Name: strings.TrimSpace(name)}
	if p.Name == "" {
		return p, fmt.Errorf("%w: name must not be empty", ErrInvalid)
	}
	err := s.store.StorePlayer(ctx, &p)
	return p, err
}

func (s *Service) CreateGame(ctx context.Context, g game.Game) (GameView, error) {
	g.ID = 0
	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	players, err := s.store.GetPlayers(ctx, g.PlayerIDs)
	if err != nil {
		return GameView{}, err
	}
	if len(players) != len(g.PlayerIDs) {
		return GameView{}, fmt.Errorf("%w: unknown player in player_ids", ErrInvalid)
	}

	if err := s.store.CreateGame(ctx, &g); err != nil {
		return GameView{}, err
	}
	return s.view(ctx, g)
}

func (s *Service) Game(ctx context.Context, id int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, id)
	if err != nil {
		return GameView{}, err
	}
	return s.view(ctx, g)
}

// ListGames returns the most recent games, newest first.
func (s *Service) ListGames(ctx context.Context, limit int) ([]GameView, error) {
	games, err := s.store.ListGames(ctx, limit)
	if err != nil {
		return nil, err
	}

	views := []GameView{}
	for _, g := range games {
		v, err := s.view(ctx, g)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}

// AddDart registers the next dart of a game. The dart is only stored if the
// game accepts it; otherwise the error wraps throw.ErrInvalid or turn.ErrGameOver.
// A missing multiplier counts as single.
func (s *Service) AddDart(ctx context.Context, gameID int64, d throw.Dart) (GameView, error) {
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}

	d.ID = 0
	d.GameID = g.ID
	if d.Miss {
		d.Number, d.Multiplier = 0, 0
	} else if d.Multiplier == 0 {
		d.Multiplier = 1
	}

	s.dartsMu.Lock()
	defer s.dartsMu.Unlock()

	darts, err := s.store.ListDarts(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}

	// The stored darts were accepted before, so an error can only come from the new one.
	if _, err := g.Play(append(darts, d)); err != nil {
		return GameView{}, err
	}

	if err := s.store.AddDart(ctx, &d); err != nil {
		return GameView{}, err
	}
	return s.view(ctx, g)
}

func (s *Service) UndoDart(ctx context.Context, gameID int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}

	s.dartsMu.Lock()
	defer s.dartsMu.Unlock()

	err = s.store.DeleteLastDart(ctx, g.ID)
	if errors.Is(err, store.ErrNotFound) {
		return GameView{}, ErrNothingToUndo
	}
	if err != nil {
		return GameView{}, err
	}
	return s.view(ctx, g)
}

func (s *Service) view(ctx context.Context, g game.Game) (GameView, error) {
	darts, err := s.store.ListDarts(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}
	state, err := g.Play(darts)
	if err != nil {
		return GameView{}, err
	}

	players, err := s.store.GetPlayers(ctx, g.PlayerIDs)
	if err != nil {
		return GameView{}, err
	}
	byID := map[int64]player.Player{}
	for _, p := range players {
		byID[p.ID] = p
	}
	ordered := []player.Player{}
	for _, id := range g.PlayerIDs {
		ordered = append(ordered, byID[id])
	}

	return GameView{Game: g, Players: ordered, Darts: darts, State: state}, nil
}
