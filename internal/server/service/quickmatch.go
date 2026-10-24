package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// QuickSettings are the rules of a quick match of the mode: one leg, no bull-off and a
// minute per turn, so a stranger can't leave the game lying; X01 from 501 with a double to
// check out, around the clock without the bull and without multiplier skips.
func QuickSettings(mode game.Mode) game.Settings {
	s := game.Settings{Mode: mode, Legs: 1, Sets: 1, TurnSeconds: 60, Rules: game.NewRules(mode)}
	if rules, ok := s.Rules.(*x01.Settings); ok {
		rules.StartScore, rules.OutMode = 501, x01.OutModeDouble
	}
	return s
}

// IsQuickMatch reports whether a game is a quick match: a single game open for two, not
// only for boards, with QuickSettings. A game opened by hand like that counts as well.
func IsQuickMatch(g game.Game) bool {
	return g.TournamentID == nil && isQuick(g.Settings, g.Lobby)
}

func isQuick(s game.Settings, l game.Lobby) bool {
	s.SettingsID = 0
	return l.Seats == 2 && !l.BoardOnly && reflect.DeepEqual(s, QuickSettings(s.Mode))
}

// QuickMatch puts the account's own player into a quick match of the mode, on the board if
// one is given: into the oldest waiting one whose player fits theirs by now (OpenLobby.Fits:
// the longer it waited, the further apart they may be), else into a new one where they wait
// for an opponent. The game starts as soon as the second player is in.
func (s *Service) QuickMatch(ctx context.Context, acc account.Account, mode game.Mode, boardID *int64) (GameView, error) {
	if game.NewRules(mode) == nil {
		return GameView{}, fmt.Errorf("%w: unknown mode %q", ErrInvalid, mode)
	}
	lobbies, err := s.quickLobbies(ctx, acc, mode, nil)
	if err != nil {
		return GameView{}, err
	}
	ids, _ := fitting(lobbies)
	switch id, ok, err := s.joinFirst(ctx, acc, ids, boardID); {
	case err != nil:
		return GameView{}, err
	case ok:
		return s.Game(ctx, acc, id)
	}
	return s.CreateGame(ctx, acc, game.Game{Settings: QuickSettings(mode), Lobby: game.Lobby{Seats: 2}}, 0, boardID)
}

// QuickMatchMove moves the account's player from the quick match they wait in alone to an
// older waiting one that fits by now, so two who wait find each other without anybody new
// coming. Only a newer one moves to an older one, so two never move into each other's. It
// returns the game the player is in afterwards (the same one if nothing fits, or the game
// isn't a quick match waiting for them alone) and when an older one will fit: forms don't
// change while someone waits, so the waiting page asks once more right then, not all the
// time. A zero time means none will.
func (s *Service) QuickMatchMove(ctx context.Context, acc account.Account, gameID int64) (int64, time.Time, error) {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return 0, time.Time{}, err
	}
	g := v.Game
	if v.Started || !IsQuickMatch(g) || len(g.Participants) != 1 || g.Participants[0].PlayerID != acc.PlayerID {
		return gameID, time.Time{}, nil
	}
	lobbies, err := s.quickLobbies(ctx, acc, g.Settings.Mode, &g)
	if err != nil {
		return 0, time.Time{}, err
	}
	ids, next := fitting(lobbies)
	if len(ids) == 0 {
		return gameID, next, nil
	}
	boardID := g.Participants[0].BoardID
	if _, err := s.LeaveGame(ctx, acc, gameID); err != nil {
		return 0, time.Time{}, err
	}
	if id, ok, err := s.joinFirst(ctx, acc, ids, boardID); err != nil || ok {
		return id, time.Time{}, err
	}
	// Someone else was faster: search again, or wait in a new one, whose page asks anew.
	v, err = s.QuickMatch(ctx, acc, g.Settings.Mode, boardID)
	return v.Game.ID, time.Time{}, err
}

// quickLobbies returns the waiting quick matches of others in the mode, oldest first; with
// older, only those older than it. Older goes by the ID, which grows strictly, unlike
// created_at (SQLite keeps whole seconds).
func (s *Service) quickLobbies(ctx context.Context, acc account.Account, mode game.Mode, older *game.Game) ([]OpenLobby, error) {
	lobbies, err := s.OpenLobbies(ctx, acc)
	if err != nil {
		return nil, err
	}
	var quick []OpenLobby
	for _, l := range lobbies {
		if !l.Tournament && l.Settings.Mode == mode && isQuick(l.Settings, game.Lobby{Seats: l.Seats, BoardOnly: l.BoardOnly}) && (older == nil || l.ID < older.ID) {
			quick = append(quick, l)
		}
	}
	slices.SortFunc(quick, func(a, b OpenLobby) int { return cmp.Compare(a.ID, b.ID) })
	return quick, nil
}

// fitting returns the IDs of the lobbies that fit by now, in their order, and when the
// first of the others will (zero if there are none).
func fitting(lobbies []OpenLobby) (ids []int64, next time.Time) {
	for _, l := range lobbies {
		switch {
		case l.Fits:
			ids = append(ids, l.ID)
		case next.IsZero() || l.FitsAt.Before(next):
			next = l.FitsAt
		}
	}
	return ids, next
}

// joinFirst joins the first of the games that still has a seat; ok is false if none had.
func (s *Service) joinFirst(ctx context.Context, acc account.Account, ids []int64, boardID *int64) (id int64, ok bool, err error) {
	for _, id := range ids {
		switch err := s.Join(ctx, acc, GameLobby(id), boardID); {
		case err == nil:
			return id, true, nil
		case !errors.Is(err, ErrNoSeat):
			return 0, false, err
		}
	}
	return 0, false, nil
}
