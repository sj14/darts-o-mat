package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// OpenFor is how long a game or tournament open for strangers is offered, from its
// creation; so lobbies nobody started don't pile up.
const OpenFor = 30 * time.Minute

// ErrNoSeat means the game or tournament has no free seat for the account's player, or
// isn't offered anymore.
var ErrNoSeat = fmt.Errorf("%w: no free seat", ErrInvalid)

// OpenLobby is a game or tournament strangers can join, as the list shows it.
type OpenLobby struct {
	Tournament bool              `json:"tournament"` // a tournament, else a game
	ID         int64             `json:"id"`
	Settings   game.Settings     `json:"settings"`
	Format     tournament.Format `json:"format,omitempty"`     // of a tournament
	Joined     []player.Player   `json:"joined"`               // who is in already
	Seats      int               `json:"seats"`                // how many it wants in total
	BoardOnly  bool              `json:"board_only,omitempty"` // everyone needs a board
	// Reputations are the thumbs of the players in Joined who got enough of them to show.
	Reputations map[int64]player.Reputation `json:"reputations,omitempty"`
	CreatedAt   time.Time                   `json:"created_at"`
}

// offered reports whether a lobby that was created at the time still takes players.
func offered(l game.Lobby, players int, createdAt time.Time, aborted bool) bool {
	return l.Open() && !l.Full(players) && !aborted && time.Since(createdAt) < OpenFor
}

// OpenLobbies lists the games and tournaments of others the account's player could join,
// newest first. Those with a player of the account (its own or a guest) aren't others'.
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

	// joined returns the players already in, nil if one of them is the account's.
	joined := func(ids []int64) ([]player.Player, error) {
		players, err := s.store.GetPlayers(ctx, ids)
		if err != nil {
			return nil, err
		}
		if slices.ContainsFunc(players, acc.Controls) {
			return nil, nil
		}
		return players, nil
	}
	var lobbies []OpenLobby
	for _, g := range games {
		if !offered(g.Lobby, len(g.Participants), g.CreatedAt, g.Aborted()) || g.TournamentID != nil {
			continue
		}
		players, err := joined(g.PlayerIDs())
		if err != nil {
			return nil, err
		}
		if players != nil {
			lobbies = append(lobbies, OpenLobby{ID: g.ID, Settings: g.Settings, Joined: players, Seats: g.Seats, BoardOnly: g.BoardOnly, CreatedAt: g.CreatedAt})
		}
	}
	for _, t := range tournaments {
		if !offered(t.Lobby, len(t.Participants), t.CreatedAt, t.Aborted()) {
			continue
		}
		players, err := joined(t.PlayerIDs())
		if err != nil {
			return nil, err
		}
		if players != nil {
			lobbies = append(lobbies, OpenLobby{Tournament: true, ID: t.ID, Settings: t.Settings, Format: t.Format, Joined: players, Seats: t.Seats, BoardOnly: t.BoardOnly, CreatedAt: t.CreatedAt})
		}
	}
	// The thumbs of whoever is in, if there are enough of them.
	var ids []int64
	for _, l := range lobbies {
		for _, p := range l.Joined {
			ids = append(ids, p.ID)
		}
	}
	reputations, err := s.store.Reputations(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i, l := range lobbies {
		for _, p := range l.Joined {
			if r := reputations[p.ID]; r.Shown() {
				if lobbies[i].Reputations == nil {
					lobbies[i].Reputations = map[int64]player.Reputation{}
				}
				lobbies[i].Reputations[p.ID] = r
			}
		}
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
	case !g.Open():
		return GameView{}, ErrNotFound
	case slices.Contains(g.PlayerIDs(), acc.PlayerID):
		return GameView{}, fmt.Errorf("%w: you are in this game already", ErrInvalid)
	case !offered(g.Lobby, len(g.Participants), g.CreatedAt, g.Aborted()):
		return GameView{}, ErrNoSeat
	}
	if err := s.checkSeatBoard(ctx, acc, gameSeat(g), boardID); err != nil {
		return GameView{}, err
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return GameView{}, err
	}
	if err := s.store.JoinGame(ctx, gameID, acc.PlayerID, boardID); err != nil {
		return GameView{}, err
	}
	if err := s.takeBoard(ctx, boardID, gameID); err != nil {
		return GameView{}, err
	}
	if err := s.startGame(ctx, gameID); err != nil {
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
	case !t.Open():
		return TournamentView{}, ErrNotFound
	case slices.Contains(t.PlayerIDs(), acc.PlayerID):
		return TournamentView{}, fmt.Errorf("%w: you are in this tournament already", ErrInvalid)
	case !offered(t.Lobby, len(t.Participants), t.CreatedAt, t.Aborted()):
		return TournamentView{}, ErrNoSeat
	}
	if err := s.checkSeatBoard(ctx, acc, tournamentSeat(t), boardID); err != nil {
		return TournamentView{}, err
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return TournamentView{}, err
	}
	if err := s.store.JoinTournament(ctx, id, acc.PlayerID, boardID); err != nil {
		return TournamentView{}, err
	}
	s.tournaments.Notify(id)
	return s.Tournament(ctx, acc, id)
}
