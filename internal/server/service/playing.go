package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
)

// PlayerBusyError means a player can't take a seat in a game or tournament because they
// have one in another: a person plays one at a time.
type PlayerBusyError struct {
	Player string
	// GameID or TournamentID is where the player has their seat, 0 if the account takes no
	// part in it (it stays hidden).
	GameID       int64
	TournamentID int64
}

func (e PlayerBusyError) Error() string {
	return fmt.Sprintf("%v: %q is in another game or tournament", ErrInvalid, e.Player)
}

func (e PlayerBusyError) Unwrap() error {
	return ErrInvalid
}

// checkPlayersFree checks that none of the players has a seat in a game or tournament
// that isn't finished: a lobby they are in (accepted, not just invited), a running game
// they didn't leave, or a tournament that is neither over nor aborted, also between its
// matches. The tournament keep (0 for none) doesn't count: StartMatch starts its matches.
// A game that is over doesn't count though it can still be corrected (going on completes
// it). The caller holds s.mu.
func (s *Service) checkPlayersFree(ctx context.Context, acc account.Account, playerIDs []int64, keep int64) error {
	for _, id := range playerIDs {
		games, err := s.store.ListUnfinishedGamesByPlayer(ctx, id)
		if err != nil {
			return err
		}
		for _, g := range games {
			v, err := s.build(ctx, g)
			if err != nil {
				return err
			}
			if v.Finished() || v.Common().Records[id].Left {
				continue
			}
			busy := PlayerBusyError{Player: playerName(v.Players, id)}
			if _, err := participationOf(acc, v.Players, v.Pending, false); err == nil {
				busy.GameID = g.ID
			}
			return busy
		}
		tournaments, err := s.store.ListUnfinishedTournamentsByPlayer(ctx, id)
		if err != nil {
			return err
		}
		for _, t := range tournaments {
			if t.ID == keep {
				continue
			}
			v, err := s.buildTournament(ctx, t)
			if err != nil {
				return err
			}
			if v.Finished() {
				continue
			}
			busy := PlayerBusyError{Player: playerName(v.Players, id)}
			if _, err := participationOf(acc, v.Players, v.Pending, false); err == nil {
				busy.TournamentID = t.ID
			}
			return busy
		}
	}
	return nil
}

// playerName is the name of the player among the players.
func playerName(players []player.Player, id int64) string {
	if i := slices.IndexFunc(players, func(p player.Player) bool { return p.ID == id }); i >= 0 {
		return players[i].Name
	}
	return ""
}
