package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
)

// OpenFor is how long a game or tournament open for strangers is offered, from its
// creation; so lobbies nobody started don't pile up.
const OpenFor = 30 * time.Minute

// ErrNoSeat means the game or tournament has no free seat for the account's player, or
// isn't offered anymore.
var ErrNoSeat = fmt.Errorf("%w: no free seat", ErrInvalid)

// OpenLobby is a game or tournament strangers can join, as the list shows it.
type OpenLobby struct {
	Tournament bool // a tournament, else a game
	ID         int64
	Host       string
	Settings   game.Settings
	Format     tournament.Format // of a tournament
	Players    int               // who is in already
	Seats      int               // how many it wants in total
	CreatedAt  time.Time
}

// offered reports whether a lobby that was created at the time still takes players.
func offered(l game.Lobby, players int, createdAt time.Time, aborted bool) bool {
	return l.Open && !l.Full(players) && !aborted && time.Since(createdAt) < OpenFor
}

// OpenLobbies lists the games and tournaments of others the account's player could join,
// newest first.
func (s *Service) OpenLobbies(ctx context.Context, acc account.Account) ([]OpenLobby, error) {
	since := time.Now().Add(-OpenFor)
	games, err := s.store.ListOpenGames(ctx, since)
	if err != nil {
		return nil, err
	}
	tournaments, err := s.store.ListOpenTournaments(ctx, since)
	if err != nil {
		return nil, err
	}

	hosts := map[int64]string{}
	host := func(id int64) (string, error) {
		if name, ok := hosts[id]; ok {
			return name, nil
		}
		a, err := s.store.GetAccount(ctx, id)
		hosts[id] = a.Username
		return a.Username, err
	}
	var lobbies []OpenLobby
	for _, g := range games {
		if !offered(g.Lobby, len(g.Participants), g.CreatedAt, g.Aborted()) || slices.Contains(g.PlayerIDs(), acc.PlayerID) || g.CreatedBy == acc.ID || g.TournamentID != nil {
			continue
		}
		h, err := host(g.CreatedBy)
		if err != nil {
			return nil, err
		}
		lobbies = append(lobbies, OpenLobby{ID: g.ID, Host: h, Settings: g.Settings, Players: len(g.Participants), Seats: g.Seats, CreatedAt: g.CreatedAt})
	}
	for _, t := range tournaments {
		if !offered(t.Lobby, len(t.Participants), t.CreatedAt, t.Aborted()) || slices.Contains(t.PlayerIDs(), acc.PlayerID) || t.CreatedBy == acc.ID {
			continue
		}
		h, err := host(t.CreatedBy)
		if err != nil {
			return nil, err
		}
		lobbies = append(lobbies, OpenLobby{Tournament: true, ID: t.ID, Host: h, Settings: t.Settings, Format: t.Format, Players: len(t.Participants), Seats: t.Seats, CreatedAt: t.CreatedAt})
	}
	slices.SortStableFunc(lobbies, func(a, b OpenLobby) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return lobbies, nil
}

// JoinGame takes a free seat in a game open for strangers with the account's own player,
// who throws on the board if one is given (one of the account's, or one other players
// throw on). Games that aren't open stay hidden (ErrNotFound).
func (s *Service) JoinGame(ctx context.Context, acc account.Account, gameID int64, boardID *int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}
	switch {
	case !g.Open:
		return GameView{}, ErrNotFound
	case slices.Contains(g.PlayerIDs(), acc.PlayerID):
		return GameView{}, fmt.Errorf("%w: you are in this game already", ErrInvalid)
	case !offered(g.Lobby, len(g.Participants), g.CreatedAt, g.Aborted()):
		return GameView{}, ErrNoSeat
	}
	if err := s.checkGameBoard(ctx, acc, g, boardID); err != nil {
		return GameView{}, err
	}
	if err := s.store.JoinGame(ctx, gameID, acc.PlayerID, boardID); err != nil {
		return GameView{}, err
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

// JoinTournament is JoinGame for a tournament; whoever takes the last seat completes
// the draw.
func (s *Service) JoinTournament(ctx context.Context, acc account.Account, id int64, boardID *int64) (TournamentView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.store.GetTournament(ctx, id)
	if err != nil {
		return TournamentView{}, err
	}
	switch {
	case !t.Open:
		return TournamentView{}, ErrNotFound
	case slices.Contains(t.PlayerIDs(), acc.PlayerID):
		return TournamentView{}, fmt.Errorf("%w: you are in this tournament already", ErrInvalid)
	case !offered(t.Lobby, len(t.Participants), t.CreatedAt, t.Aborted()):
		return TournamentView{}, ErrNoSeat
	}
	if err := s.checkTournamentBoard(ctx, acc, t, boardID); err != nil {
		return TournamentView{}, err
	}
	if err := s.store.JoinTournament(ctx, id, acc.PlayerID, boardID); err != nil {
		return TournamentView{}, err
	}
	s.tournaments.Notify(id)
	return s.Tournament(ctx, acc, id)
}
