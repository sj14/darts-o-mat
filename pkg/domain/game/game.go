package game

import (
	"errors"
	"fmt"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type Mode string

const (
	ModeX01            Mode = "x01"
	ModeAroundTheClock Mode = "around_the_clock"
)

// Game holds what is needed to replay a game: the mode, its settings and the players in throwing order.
// The darts are stored separately and the current state is computed with Play.
type Game struct {
	ID             int64                    `bun:",pk,autoincrement" json:"id"`
	OwnerAccountID int64                    `bun:",notnull" json:"owner_account_id"` // the account that created the game
	BoardID        *int64                   `json:"board_id,omitempty"`
	Mode           Mode                     `bun:",notnull" json:"mode"`
	PlayerIDs      []int64                  `bun:"player_ids,notnull" json:"player_ids"`
	X01            *x01.Settings            `json:"x01,omitempty"`
	AroundTheClock *aroundtheclock.Settings `json:"around_the_clock,omitempty"`
	CreatedAt      time.Time                `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

func (g Game) Validate() error {
	if len(g.PlayerIDs) == 0 {
		return errors.New("a game needs at least one player")
	}
	seen := map[int64]bool{}
	for _, id := range g.PlayerIDs {
		if seen[id] {
			return fmt.Errorf("player %d is listed twice", id)
		}
		seen[id] = true
	}

	switch g.Mode {
	case ModeX01:
		if g.X01 == nil || g.AroundTheClock != nil {
			return errors.New("mode x01 needs the x01 settings only")
		}
		return g.X01.Validate()
	case ModeAroundTheClock:
		if g.AroundTheClock == nil || g.X01 != nil {
			return errors.New("mode around_the_clock needs the around_the_clock settings only")
		}
		return g.AroundTheClock.Validate()
	default:
		return fmt.Errorf("unknown mode %q, use x01 or around_the_clock", g.Mode)
	}
}

// Play replays the darts and returns the state of the game mode,
// x01.State or aroundtheclock.State.
func (g Game) Play(darts []throw.Dart) (any, error) {
	switch g.Mode {
	case ModeX01:
		return x01.Play(*g.X01, g.PlayerIDs, darts)
	case ModeAroundTheClock:
		return aroundtheclock.Play(*g.AroundTheClock, g.PlayerIDs, darts)
	default:
		return nil, fmt.Errorf("unknown mode %q", g.Mode)
	}
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
)

// Invitation invites the player of another account to a game.
// The game accepts darts once no invitation is pending anymore;
// a declined or withdrawn invitation is deleted and the player removed from the game.
type Invitation struct {
	GameID    int64            `bun:",pk" json:"game_id"`
	PlayerID  int64            `bun:",pk" json:"player_id"`
	Status    InvitationStatus `bun:",notnull" json:"status"`
	CreatedAt time.Time        `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}
