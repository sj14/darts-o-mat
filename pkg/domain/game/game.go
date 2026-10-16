package game

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// MaxLegs is the most legs a game (or a set) can need to win, MaxSets the most sets.
const (
	MaxLegs = 11
	MaxSets = 7
)

type Mode string

const (
	ModeX01            Mode = "x01"
	ModeAroundTheClock Mode = "around_the_clock"
)

// Game holds what is needed to replay a game: the mode, its settings and the players in throwing order.
// The darts are stored separately and the current state is computed with Play.
type Game struct {
	ID             int64   `bun:",pk,autoincrement" json:"id"`
	OwnerAccountID int64   `bun:",notnull" json:"owner_account_id"` // the account that created the game
	BoardID        *int64  `json:"board_id,omitempty"`
	Mode           Mode    `bun:",notnull" json:"mode"`
	PlayerIDs      []int64 `bun:"player_ids,notnull" json:"player_ids"`
	// BullOff lets the players bull off for who starts the first leg.
	BullOff bool `bun:",notnull,default:false" json:"bull_off"`
	// BullOffCloser is the player the players found closer when bulling off ended in a
	// tie, see turn.Log.BullOffCloser.
	BullOffCloser *int64 `json:"bull_off_closer,omitempty"`
	// Legs is how many legs win the game, or a set when playing sets; at least 1.
	Legs int `bun:",notnull,default:1" json:"legs"`
	// Sets is how many sets win the game; 1 is a game without sets.
	Sets           int                      `bun:",notnull,default:1" json:"sets"`
	X01            *x01.Settings            `json:"x01,omitempty"`
	AroundTheClock *aroundtheclock.Settings `json:"around_the_clock,omitempty"`
	CreatedAt      time.Time                `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	// AbortedAt is set when the owner aborted the game: it takes no more darts and has no winner.
	AbortedAt *time.Time `bun:",nullzero" json:"aborted_at,omitempty"`
	// TournamentID is the tournament the game is a match of (TournamentMatch, the index
	// in its schedule), nil for a single game.
	TournamentID    *int64 `json:"tournament_id,omitempty"`
	TournamentMatch int    `bun:",notnull,default:0" json:"tournament_match,omitempty"`
}

// Aborted reports whether the game was aborted.
func (g Game) Aborted() bool {
	return g.AbortedAt != nil
}

func (g Game) Validate() error {
	if len(g.PlayerIDs) == 0 {
		return errors.New("a game needs at least one player")
	}
	if g.Legs < 1 || g.Legs > MaxLegs {
		return fmt.Errorf("legs must be between 1 and %d", MaxLegs)
	}
	if g.Sets < 1 || g.Sets > MaxSets {
		return fmt.Errorf("sets must be between 1 and %d", MaxSets)
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

// WaitsForTakeout reports whether a finished turn waits until the darts were pulled out.
// Games on a board do: its cameras report the takeout, players without them confirm it.
func (g Game) WaitsForTakeout() bool {
	return g.BoardID != nil
}

// State is a replayed game's state, of its mode's type: x01.State or
// aroundtheclock.State. The part all modes share is its turn.State.
type State interface {
	Common() turn.State
}

// Play replays the game's log (darts and takeouts, as stored) and returns its state.
func (g Game) Play(entries []throw.Dart) (State, error) {
	log := turn.Log{Entries: entries, WaitForTakeout: g.WaitsForTakeout(), Legs: g.Legs, Sets: g.Sets, BullOff: g.BullOff}
	if g.BullOffCloser != nil {
		if i := slices.Index(g.PlayerIDs, *g.BullOffCloser); i >= 0 {
			log.BullOffCloser = &i
		}
	}
	switch g.Mode {
	case ModeX01:
		return x01.PlayLog(*g.X01, g.PlayerIDs, log)
	case ModeAroundTheClock:
		return aroundtheclock.PlayLog(*g.AroundTheClock, g.PlayerIDs, log)
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

// Score returns what each player has won in the game's state (see Play): sets in a
// game with sets, legs otherwise.
func (g Game) Score(state State) map[int64]int {
	score := map[int64]int{}
	for id, r := range state.Common().Records {
		score[id] = r.Legs
		if g.Sets > 1 {
			score[id] = r.Sets
		}
	}
	return score
}
