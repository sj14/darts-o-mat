package service

import (
	"context"
	"fmt"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
)

// PlayerBusyError means a player can't take part in a game because they play in another
// running one: a person throws in one game at a time.
type PlayerBusyError struct {
	Player string
	// GameID is the running game, 0 if the account takes no part in it (it stays hidden).
	GameID int64
}

func (e PlayerBusyError) Error() string {
	return fmt.Sprintf("%v: %q plays in another game", ErrInvalid, e.Player)
}

func (e PlayerBusyError) Unwrap() error {
	return ErrInvalid
}

// checkPlayersFree checks that none of the players takes part in a running game: started,
// neither over nor aborted, and not left by them. A lobby doesn't count, nor does a
// tournament between its matches. The caller holds s.mu.
func (s *Service) checkPlayersFree(ctx context.Context, acc account.Account, playerIDs []int64) error {
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
			busy := PlayerBusyError{}
			if i := slices.IndexFunc(v.Players, func(p player.Player) bool { return p.ID == id }); i >= 0 {
				busy.Player = v.Players[i].Name
			}
			if _, err := participationOf(acc, v.Players, v.Pending, false); err == nil {
				busy.GameID = g.ID
			}
			return busy
		}
	}
	return nil
}
