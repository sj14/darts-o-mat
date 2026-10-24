package service

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

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
	// Forms are how well the players in Joined play the lobby's mode lately (stats.Form),
	// of those with enough completed games.
	Forms map[int64]float64 `json:"forms,omitempty"`
	// FitsAt is when the players' form is close enough to that of the account's own player,
	// as the lobby waits (see fitsAt): its creation without a form on either side. Fits says
	// whether that time has come.
	FitsAt    time.Time `json:"fits_at"`
	Fits      bool      `json:"fits"`
	CreatedAt time.Time `json:"created_at"`
}

// offered reports whether a lobby still takes players. It does as long as someone is in
// it: one who has a seat can't take another (checkPlayersFree), so lobbies don't pile up,
// and one that everybody left is deleted.
func offered(l game.Lobby, players int, aborted bool) bool {
	return l.Open() && !l.Full(players) && !aborted
}

// OpenLobbies lists the games and tournaments of others the account's player could join,
// newest first. Those with a player of the account (its own or a guest) aren't others'.
func (s *Service) OpenLobbies(ctx context.Context, acc account.Account) ([]OpenLobby, error) {
	games, err := s.store.ListOpenGames(ctx)
	if err != nil {
		return nil, err
	}
	tournaments, err := s.store.ListOpenTournaments(ctx)
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
		if !offered(g.Lobby, len(g.Participants), g.Aborted()) || g.TournamentID != nil {
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
		if !offered(t.Lobby, len(t.Participants), t.Aborted()) {
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
	if err := s.addForms(ctx, acc, lobbies); err != nil {
		return nil, err
	}
	slices.SortStableFunc(lobbies, func(a, b OpenLobby) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return lobbies, nil
}

// addForms fills in the forms of the players in the lobbies, in each lobby's mode, and
// whether each lobby fits the account's own player (OpenLobby.Fits).
func (s *Service) addForms(ctx context.Context, acc account.Account, lobbies []OpenLobby) error {
	ids := map[game.Mode][]int64{}
	for _, l := range lobbies {
		if len(ids[l.Settings.Mode]) == 0 {
			ids[l.Settings.Mode] = []int64{acc.PlayerID}
		}
		for _, p := range l.Joined {
			ids[l.Settings.Mode] = append(ids[l.Settings.Mode], p.ID)
		}
	}
	forms := map[game.Mode]map[int64]float64{}
	for mode, players := range ids {
		results, err := s.store.ListRecentResults(ctx, players, mode, stats.FormGames)
		if err != nil {
			return err
		}
		byPlayer := map[int64][]stats.Result{}
		for _, r := range results {
			byPlayer[r.PlayerID] = append(byPlayer[r.PlayerID], r)
		}
		forms[mode] = map[int64]float64{}
		for id, rs := range byPlayer {
			if form, ok := stats.Form(mode, rs); ok {
				forms[mode][id] = form
			}
		}
	}
	for i, l := range lobbies {
		for _, p := range l.Joined {
			if form, ok := forms[l.Settings.Mode][p.ID]; ok {
				if lobbies[i].Forms == nil {
					lobbies[i].Forms = map[int64]float64{}
				}
				lobbies[i].Forms[p.ID] = form
			}
		}
		lobbies[i].FitsAt = l.CreatedAt
		if mine, ok := forms[l.Settings.Mode][acc.PlayerID]; ok {
			lobbies[i].FitsAt = lobbies[i].fitsAt(mine)
		}
		lobbies[i].Fits = !s.now().Before(lobbies[i].FitsAt)
	}
	return nil
}

// fitsAt is when the form of a player fits the lobby: once it has waited long enough
// (stats.FormWait) for the average of the forms of its players who have one. A lobby of
// players without a form fits anyone from the start.
func (l OpenLobby) fitsAt(form float64) time.Time {
	if len(l.Forms) == 0 {
		return l.CreatedAt
	}
	var sum float64
	for _, f := range l.Forms {
		sum += f
	}
	return l.CreatedAt.Add(stats.FormWait(l.Settings.Mode, form, sum/float64(len(l.Forms))))
}
