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
// single game, and more than 16 don't fit on a screen.
const (
	MinPlayers = 3
	MaxPlayers = 16
)

// Tournament holds what is needed to compute its state: the format, the settings of
// its games and the players in the order of the draw (the seeds of the bracket).
type Tournament struct {
	ID             int64  `bun:",pk,autoincrement" json:"id"`
	OwnerAccountID int64  `bun:",notnull" json:"owner_account_id"`
	Format         Format `bun:",notnull" json:"format"`
	// Settings are the settings of its games: mode, legs, sets, bull-off, board and the
	// mode's settings. ID, owner and players are set per game.
	Settings  game.Game  `bun:",notnull" json:"settings"`
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
	// The settings must make a valid game, with the players of the tournament.
	g := t.Settings
	g.PlayerIDs = t.PlayerIDs
	return g.Validate()
}

// Game returns a new game of the match between the players a and b.
func (t Tournament) Game(match int, a, b int64) game.Game {
	g := t.Settings
	g.ID, g.OwnerAccountID, g.PlayerIDs = 0, t.OwnerAccountID, []int64{a, b}
	g.CreatedAt, g.AbortedAt, g.BullOffCloser = time.Time{}, nil, nil
	g.TournamentID, g.TournamentMatch = &t.ID, match
	return g
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
