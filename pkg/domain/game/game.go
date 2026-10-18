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

	"github.com/uptrace/bun"
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

// Settings are how a game is played: the mode with its settings, legs, sets, bulling
// off. A tournament has them once for all its games. Both store them as
// columns of their own table, except the mode's settings (JSON).
type Settings struct {
	Mode Mode `bun:",notnull" json:"mode"`
	// Legs is how many legs win the game, or a set when playing sets; at least 1.
	Legs int `bun:",notnull,default:1" json:"legs"`
	// Sets is how many sets win the game; 1 is a game without sets.
	Sets int `bun:",notnull,default:1" json:"sets"`
	// BullOff lets the players bull off for who starts the first leg.
	BullOff        bool                     `bun:",notnull,default:false" json:"bull_off"`
	X01            *x01.Settings            `json:"x01,omitempty"`
	AroundTheClock *aroundtheclock.Settings `json:"around_the_clock,omitempty"`
}

func (s Settings) Validate() error {
	if s.Legs < 1 || s.Legs > MaxLegs {
		return fmt.Errorf("legs must be between 1 and %d", MaxLegs)
	}
	if s.Sets < 1 || s.Sets > MaxSets {
		return fmt.Errorf("sets must be between 1 and %d", MaxSets)
	}
	switch s.Mode {
	case ModeX01:
		if s.X01 == nil || s.AroundTheClock != nil {
			return errors.New("mode x01 needs the x01 settings only")
		}
		return s.X01.Validate()
	case ModeAroundTheClock:
		if s.AroundTheClock == nil || s.X01 != nil {
			return errors.New("mode around_the_clock needs the around_the_clock settings only")
		}
		return s.AroundTheClock.Validate()
	default:
		return fmt.Errorf("unknown mode %q, use x01 or around_the_clock", s.Mode)
	}
}

// MaxSeats is the most players of a game that is open for strangers.
const MaxSeats = 8

// Lobby makes a game or tournament open for players without an invitation: Seats is the
// number of players it wants in total, the creator's and invited ones included; 0 means
// it isn't open. It starts once all seats are taken and the invited players accepted.
// Strangers see it in the list of open games until then, see service.OpenLobbies.
type Lobby struct {
	Seats int `bun:",notnull,default:0" json:"seats,omitempty"`
}

// Open reports whether strangers can join.
func (l Lobby) Open() bool {
	return l.Seats > 0
}

// Full reports whether no seat is free with that many players; a game that isn't open
// always is.
func (l Lobby) Full(players int) bool {
	return !l.Open() || players >= l.Seats
}

// Validate checks the lobby of a game or tournament that has players players so far and
// may have min to max in total.
func (l Lobby) Validate(players, min, max int) error {
	switch {
	case l.Seats < 0:
		return errors.New("seats must not be negative")
	case !l.Open():
		return nil
	case l.Seats < min || l.Seats > max:
		return fmt.Errorf("a game open for strangers needs %d to %d players", min, max)
	case players >= l.Seats:
		return errors.New("a game open for strangers needs a free seat")
	}
	return nil
}

// Participant is a player of a game: its place in the throwing order, whether the player
// accepted the invitation (the players of the account that creates a game always have),
// and the board they throw on. All players of a game are equal; whoever controls a player
// (their account, or for a guest the account they are a guest of) enters their darts.
type Participant struct {
	bun.BaseModel `bun:"table:game_players"`

	GameID   int64 `bun:",pk" json:"-"`
	PlayerID int64 `bun:",pk" json:"player_id"`
	// Position is the place in the throwing order, counting up from 0 (not necessarily
	// without gaps after a player left the lobby).
	Position int              `bun:"place,notnull" json:"position"`
	Status   InvitationStatus `bun:",notnull" json:"status"`
	// BoardID is the board the player throws on: its cameras report their darts. Nil: they
	// enter their darts by hand.
	BoardID   *int64    `json:"board_id,omitempty"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
}

// Game holds what is needed to replay a game: its settings and the players in throwing
// order. The darts are stored separately and the current state is computed with Play.
type Game struct {
	bun.BaseModel `bun:"table:games"`

	ID int64 `bun:",pk,autoincrement" json:"id"`
	Settings
	Lobby
	// Participants are the players in throwing order (see PlayerIDs). They are stored in a
	// table of their own and filled in when the game is loaded.
	Participants []Participant `bun:"-" json:"participants"`
	// BullOffCloser is the player the players found closer when bulling off ended in a
	// tie, see turn.Log.BullOffCloser.
	BullOffCloser *int64    `json:"bull_off_closer,omitempty"`
	CreatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	// AbortedAt is set when the game ended without a winner, because too many players left
	// it: it takes no more darts.
	AbortedAt *time.Time `bun:",nullzero" json:"aborted_at,omitempty"`
	// TournamentID is the tournament the game is a match of (TournamentMatch, the index
	// in its schedule), nil for a single game.
	TournamentID    *int64 `json:"tournament_id,omitempty"`
	TournamentMatch int    `bun:",notnull,default:0" json:"tournament_match,omitempty"`
}

// PlayerIDs returns the players in throwing order.
func (g Game) PlayerIDs() []int64 {
	ids := make([]int64, len(g.Participants))
	for i, p := range g.Participants {
		ids[i] = p.PlayerID
	}
	return ids
}

// Participant returns the participant with the player ID.
func (g Game) Participant(playerID int64) (Participant, bool) {
	for _, p := range g.Participants {
		if p.PlayerID == playerID {
			return p, true
		}
	}
	return Participant{}, false
}

// Aborted reports whether the game was aborted.
func (g Game) Aborted() bool {
	return g.AbortedAt != nil
}

func (g Game) Validate() error {
	if len(g.Participants) == 0 {
		return errors.New("a game needs at least one player")
	}
	if err := CheckPlayers(g.PlayerIDs()); err != nil {
		return err
	}
	if err := g.Lobby.Validate(len(g.Participants), 2, MaxSeats); err != nil {
		return err
	}
	return g.Settings.Validate()
}

// CheckPlayers checks that no player is listed twice, e.g. in a game or a tournament.
func CheckPlayers(playerIDs []int64) error {
	seen := map[int64]bool{}
	for _, id := range playerIDs {
		if seen[id] {
			return fmt.Errorf("player %d is listed twice", id)
		}
		seen[id] = true
	}
	return nil
}

// WaitsForTakeout reports whether a finished turn waits until the darts were pulled out.
// Games with a board do: its cameras report the takeout, players without them confirm it.
func (g Game) WaitsForTakeout() bool {
	return slices.ContainsFunc(g.Participants, func(p Participant) bool { return p.BoardID != nil })
}

// State is a replayed game's state, of its mode's type: x01.State or
// aroundtheclock.State. The part all modes share is its turn.State.
type State interface {
	Common() turn.State
}

// Play replays the game's log (darts and takeouts, as stored) and returns its state.
func (g Game) Play(entries []throw.Dart) (State, error) {
	log := turn.Log{Entries: entries, PlayerIDs: g.PlayerIDs(), WaitForTakeout: g.WaitsForTakeout(), Legs: g.Legs, Sets: g.Sets, BullOff: g.BullOff}
	if g.BullOffCloser != nil {
		if i := slices.Index(g.PlayerIDs(), *g.BullOffCloser); i >= 0 {
			log.BullOffCloser = &i
		}
	}
	switch g.Mode {
	case ModeX01:
		return x01.PlayLog(*g.X01, g.PlayerIDs(), log)
	case ModeAroundTheClock:
		return aroundtheclock.PlayLog(*g.AroundTheClock, g.PlayerIDs(), log)
	default:
		return nil, fmt.Errorf("unknown mode %q", g.Mode)
	}
}

type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
)

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
