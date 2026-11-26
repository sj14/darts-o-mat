package service

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/bot"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
)

// botPause is how long a bot waits before each dart and before pulling its darts out, so
// the others can follow its turn (Service.botPause).
const botPause = 1500 * time.Millisecond

// botMove returns when a bot of the game does its next move, false if none is to come: it
// throws when it is up, and in a game that waits for the takeout it pulls its own darts
// out, which aren't in a real board. The time counts from the last entry of the log (the
// start of the game before the first), so every replica works out the same; the game's
// turn timer fires then (watchTurn).
func (s *Service) botMove(v GameView) (time.Time, bool) {
	c := v.Common()
	if !v.Started || v.Game.Aborted() || c.GameOver || c.CurrentPlayerID == nil {
		return time.Time{}, false
	}
	var thrower int64
	if c.WaitingForTakeout {
		thrower = lastThrower(v)
	} else {
		thrower = *c.CurrentPlayerID
	}
	if !v.Game.IsBot(thrower) {
		return time.Time{}, false
	}
	since := *v.Game.StartedAt
	if len(v.Darts) > 0 {
		since = v.Darts[len(v.Darts)-1].CreatedAt
	}
	return since.Add(s.botPause), true
}

// lastThrower returns the player who threw the last dart of the log, 0 without one.
func lastThrower(v GameView) int64 {
	for i := len(v.Darts) - 1; i >= 0; i-- {
		if v.Darts[i].IsDart() {
			return v.Darts[i].PlayerID
		}
	}
	return 0
}

// moveBot does the move of a bot that botMove found due: it pulls its darts out, or
// throws its next dart at the field the mode tells (game.Game.Aim), spread by its seat's
// spread (bot.Throw). The caller holds the lock (s.lock) and tells the game's subscribers.
func (s *Service) moveBot(ctx context.Context, g *game.Game, v GameView) error {
	if v.Common().WaitingForTakeout {
		return s.storeTakeout(ctx, g, v)
	}
	aim, ok := g.Aim(v.State)
	if !ok {
		return nil
	}
	seat, _ := g.Participant(*v.Common().CurrentPlayerID)
	return s.storeDart(ctx, g, v, bot.Throw(s.bots, aim, seat.BotSpreadUM))
}
