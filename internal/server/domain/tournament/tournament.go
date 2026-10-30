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

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
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

// Participant is a player of a tournament, like game.Participant of a game: the place in
// the order of the draw (the seeds of the bracket), whether the player accepted, and the
// board they throw on in the matches they play.
type Participant struct {
	bun.BaseModel `bun:"table:tournament_players"`

	TournamentID int64                 `bun:",pk" json:"-"`
	PlayerID     int64                 `bun:",pk" json:"player_id"`
	Position     int                   `bun:"place,notnull" json:"position"`
	Status       game.InvitationStatus `bun:",notnull" json:"status"`
	BoardID      *int64                `json:"board_id,omitempty"`
	CreatedAt    time.Time             `bun:",nullzero,notnull,default:current_timestamp" json:"-"`
}

// Tournament holds what is needed to compute its state: the format, the settings of
// its games and the players in the order of the draw (the seeds of the bracket).
type Tournament struct {
	bun.BaseModel `bun:"table:tournaments"`

	ID     int64  `bun:",pk,autoincrement" json:"id"`
	Format Format `bun:",notnull" json:"format"`
	// Settings are the settings of all its games.
	game.Settings
	// Lobby opens the tournament for strangers, who take the seats that are left.
	game.Lobby
	// Participants are the players in the order of the draw; stored in a table of their
	// own and filled in when the tournament is loaded.
	Participants []Participant `bun:"-" json:"participants"`
	CreatedAt    time.Time     `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	// AbortedAt is set when the tournament ended without a winner, because a player left it.
	AbortedAt *time.Time `bun:",nullzero" json:"aborted_at,omitempty"`
	// StartedAt is when the tournament started: a player started it in the lobby, or the
	// last seat of an open one was taken. The order of the players is drawn then. Nil while
	// it waits in the lobby.
	StartedAt *time.Time `bun:",nullzero" json:"started_at,omitempty"`
	// CompletedAt is set once the tournament was seen completed: over, and all its games
	// completed (State.Completed); then nothing about it can change anymore.
	CompletedAt *time.Time `bun:",nullzero" json:"completed_at,omitempty"`
}

// PlayerIDs returns the players in the order of the draw.
func (t Tournament) PlayerIDs() []int64 {
	ids := make([]int64, len(t.Participants))
	for i, p := range t.Participants {
		ids[i] = p.PlayerID
	}
	return ids
}

// Participant returns the participant with the player ID.
func (t Tournament) Participant(playerID int64) (Participant, bool) {
	for _, p := range t.Participants {
		if p.PlayerID == playerID {
			return p, true
		}
	}
	return Participant{}, false
}

// Started reports whether the tournament left the lobby: its order is drawn, its games can
// start.
func (t Tournament) Started() bool {
	return t.StartedAt != nil
}

// CanStart reports whether the tournament in the lobby may start, like game.Game.CanStart;
// it also needs MinPlayers for its schedule (CheckPlayerCount).
func (t Tournament) CanStart() bool {
	statuses := make([]game.InvitationStatus, len(t.Participants))
	for i, p := range t.Participants {
		statuses[i] = p.Status
	}
	return !t.Started() && t.Lobby.CanStart(statuses)
}

// CheckPlayerCount checks that the tournament has as many players as its schedule needs
// (ErrPlayerCount). In the lobby it may have fewer.
func (t Tournament) CheckPlayerCount() error {
	if len(t.Participants) < MinPlayers || len(t.Participants) > MaxPlayers {
		return fmt.Errorf("%w: %d to %d", ErrPlayerCount, MinPlayers, MaxPlayers)
	}
	return nil
}

// Aborted reports whether the tournament ended without a winner.
func (t Tournament) Aborted() bool {
	return t.AbortedAt != nil
}

func (t Tournament) Validate() error {
	if t.Format != Knockout && t.Format != RoundRobin {
		return fmt.Errorf("unknown format %q, use knockout or round_robin", t.Format)
	}
	// In the lobby it may have fewer players than it needs; CanStart checks them.
	if len(t.Participants) > MaxPlayers {
		return fmt.Errorf("%w: %d to %d", ErrPlayerCount, MinPlayers, MaxPlayers)
	}
	if err := game.CheckPlayers(t.PlayerIDs()); err != nil {
		return err
	}
	if err := t.Lobby.Validate(t.Settings, len(t.Participants), MinPlayers, MaxPlayers); err != nil {
		return err
	}
	return t.Settings.Validate()
}

// Game returns a new game of the match between the players a and b, with the boards they
// throw on. Its players accepted the tournament, so they don't have to accept the game.
func (t Tournament) Game(match int, a, b int64) game.Game {
	g := game.Game{
		Settings:     t.Settings,
		TournamentID: &t.ID, TournamentMatch: match,
	}
	for _, id := range []int64{a, b} {
		p, _ := t.Participant(id)
		g.Participants = append(g.Participants, game.Participant{PlayerID: id, Status: game.InvitationAccepted, BoardID: p.BoardID})
	}
	return g
}

var (
	ErrNoMatch     = errors.New("no such match")
	ErrPlayerCount = errors.New("wrong number of players for a tournament")
)
