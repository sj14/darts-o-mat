package service

import (
	"context"
	"fmt"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
)

// ErrNotRateable means the player can't be given a thumb: not a player of the game with an
// account, or oneself.
var ErrNotRateable = fmt.Errorf("%w: this player can't be rated", ErrInvalid)

// Rateable returns the players the viewer can give a thumb after the game: the other
// players of it with an account (not guests), once it is over. Empty for a viewer whose
// own player isn't in the game.
func (v GameView) Rateable() []player.Player {
	if v.OwnPlayerID == 0 || !v.Common().GameOver || v.Game.Aborted() || v.Invited {
		return nil
	}
	var players []player.Player
	for _, p := range v.Players {
		if p.ID != v.OwnPlayerID && !p.IsGuest() && !p.Deleted() {
			players = append(players, p)
		}
	}
	return players
}

// RatePlayer gives another player of a finished game a thumb up or down (user's wish
// 2026-10-20, to know who one plays with, shown in the list of open games). It replaces
// the thumb the account gave that player in this game before.
func (s *Service) RatePlayer(ctx context.Context, acc account.Account, gameID, ratedID int64, up bool) (GameView, error) {
	v, err := s.Game(ctx, acc, gameID)
	switch {
	case err != nil:
		return GameView{}, err
	case v.Game.Aborted():
		return GameView{}, ErrAborted
	case !v.Common().GameOver:
		return GameView{}, ErrNotOver
	}
	rateable := false
	for _, p := range v.Rateable() {
		rateable = rateable || p.ID == ratedID
	}
	if !rateable {
		return GameView{}, ErrNotRateable
	}
	if err := s.store.SetRating(ctx, player.Rating{GameID: gameID, RaterID: acc.PlayerID, RatedID: ratedID, Up: up}); err != nil {
		return GameView{}, err
	}
	return s.Game(ctx, acc, gameID)
}
