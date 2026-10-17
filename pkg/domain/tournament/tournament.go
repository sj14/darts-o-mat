// Package tournament plays a game mode as a tournament: the players meet in single
// games (matches), either in a knockout bracket or all against all.
//
// Like a game, a tournament only stores its settings and players; the schedule, the
// bracket or table and the winner are computed from the results of its games (Plan).
package tournament

import (
	"errors"
	"fmt"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/uptrace/bun"
)

// Format is how the players meet.
type Format string

const (
	// Knockout: the loser of a match is out, the winner meets the winner of the
	// neighbouring match. Players without an opponent in the first round (with a
	// number of players that isn't a power of two) get a bye.
	Knockout Format = "knockout"
	// RoundRobin: everyone plays everyone once; the table decides.
	RoundRobin Format = "round_robin"
)

// MinPlayers and MaxPlayers limit the players of a tournament: with two it is a
// single game; 32 is a knockout bracket of five rounds (user's choice, 2026-10-14).
const (
	MinPlayers = 3
	MaxPlayers = 32
)

// Tournament holds what is needed to compute its state: the format, the settings of
// its games and the players in the order of the draw (the seeds of the bracket).
type Tournament struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	OwnerAccountID int64  `bun:",notnull" json:"owner_account_id"`
	Format         Format `bun:",notnull" json:"format"`
	// Settings are the settings of all its games.
	game.Settings
	PlayerIDs []int64    `bun:"player_ids,notnull" json:"player_ids"`
	CreatedAt time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	AbortedAt *time.Time `bun:",nullzero" json:"aborted_at,omitempty"`
}

// Aborted reports whether the owner aborted the tournament.
func (t Tournament) Aborted() bool {
	return t.AbortedAt != nil
}

func (t Tournament) Validate() error {
	if t.Format != Knockout && t.Format != RoundRobin {
		return fmt.Errorf("unknown format %q, use knockout or round_robin", t.Format)
	}
	if len(t.PlayerIDs) < MinPlayers || len(t.PlayerIDs) > MaxPlayers {
		return fmt.Errorf("%w: %d to %d", ErrPlayerCount, MinPlayers, MaxPlayers)
	}
	if err := game.CheckPlayers(t.PlayerIDs); err != nil {
		return err
	}
	return t.Settings.Validate()
}

// Game returns a new game of the match between the players a and b.
func (t Tournament) Game(match int, a, b int64) game.Game {
	return game.Game{
		OwnerAccountID: t.OwnerAccountID,
		Settings:       t.Settings,
		PlayerIDs:      []int64{a, b},
		TournamentID:   &t.ID, TournamentMatch: match,
	}
}

// Invitation invites the player of another account to a tournament. Its games start
// once no invitation is pending anymore; a declined invitation is deleted and the
// player removed from the tournament.
type Invitation struct {
	bun.BaseModel `bun:"table:tournament_invitations"`

	TournamentID int64                 `bun:",pk" json:"tournament_id"`
	PlayerID     int64                 `bun:",pk" json:"player_id"`
	Status       game.InvitationStatus `bun:",notnull" json:"status"`
	CreatedAt    time.Time             `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

var (
	ErrNoMatch     = errors.New("no such match")
	ErrPlayerCount = errors.New("wrong number of players for a tournament")
)
