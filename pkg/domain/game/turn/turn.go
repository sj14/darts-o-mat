// Package turn contains the order of play shared by all game modes: the players
// throw three darts each, one after another.
//
// A game is played over legs. The player who finishes wins the leg at once; whoever
// first wins the agreed number of legs wins the game. Each leg is started by the next
// player in turn.
//
// Optionally the players bull off first: each throws one dart at the bull, the closest
// starts the first leg. Players who are equally close throw again.
//
// Games played on a board with cameras wait after each turn until the darts were
// pulled out (a takeout), so the next player doesn't start while the darts are
// still in the board.
package turn

import (
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

const DartsPerTurn = 3

var (
	ErrGameOver = errors.New("game is already over")
	// ErrWaitingForTakeout means the turn is over, but the darts haven't been pulled out yet.
	ErrWaitingForTakeout = errors.New("pull out the darts first")
)

// Log is what happened in a game, the input to replay it.
type Log struct {
	// Entries are the darts and takeouts (throw.Dart.Takeout) in the order they happened.
	Entries []throw.Dart
	// WaitForTakeout keeps a finished turn until the darts were pulled out.
	WaitForTakeout bool
	// Legs is how many legs win the game; 0 counts as 1.
	Legs int
	// BullOff lets the players bull off for who starts (with more than one player).
	BullOff bool
	// BullOffCloser is the index of the player the players found closer when bulling
	// off ended in a tie (e.g. both hit the bull, entered by hand), nil if they didn't
	// decide. It takes the place of throwing again.
	BullOffCloser *int
}

// Turn returns the order of play for the log's settings.
func (l Log) Turn(players int) *Turn {
	t := &Turn{Round: 1, Leg: 1, LegsWon: make([]int, players), BestLeg: make([]int, players), legDarts: make([]int, players), legs: max(l.Legs, 1), waitForTakeout: l.WaitForTakeout}
	if l.BullOff && players > 1 {
		t.bull = newBullOff(players, l.BullOffCloser)
	}
	return t
}

// Replay calls dart for every dart of the log in order, except those thrown to bull
// off, and handles the takeouts in between. A dart while the turn waits for the
// takeout is an error. Afterwards t.Throwers tells who threw each entry.
func (l Log) Replay(t *Turn, dart func(i int, d throw.Dart) error) error {
	t.Throwers = make([]int, len(l.Entries))
	for i, d := range l.Entries {
		t.Throwers[i] = t.Player
		switch {
		case d.Takeout:
			// Bull-off darts are pulled out whenever the players like.
			t.Throwers[i] = -1
			if !t.BullingOff() {
				t.Takeout()
			}
			continue
		case t.Waiting():
			return fmt.Errorf("entry %d: %w", i+1, ErrWaitingForTakeout)
		case t.Over():
			return fmt.Errorf("dart %d: %w", i+1, ErrGameOver)
		case t.BullingOff():
			if err := d.Validate(); err != nil {
				return fmt.Errorf("dart %d: %w", i+1, err)
			}
			next, decided := t.bull.throw(i, t.Player, d)
			t.Player = next
			if decided {
				t.starter = next
			}
			continue
		}
		t.legWinner = nil
		if err := dart(i, d); err != nil {
			return err
		}
	}
	return nil
}

// afterTakeout is what happens when the darts of a finished turn are pulled out.
type afterTakeout uint8

const (
	nothing    afterTakeout = iota // not waiting
	nextPlayer                     // the next player throws
	nextLeg                        // the turn won a leg, the next leg starts
)

type Turn struct {
	Player  int   // index of the player whose turn it is; the winner once the game is over
	Round   int   // of the current leg, starts at 1
	Dart    int   // darts already thrown in this turn
	Leg     int   // starts at 1
	LegsWon []int // per player
	BestLeg []int // per player: fewest darts in a won leg, 0 without one
	// OnLegStart is called when the next leg starts, so the game mode resets the scores.
	OnLegStart func()
	// Throwers is the index of the player who threw each log entry, -1 for takeouts.
	Throwers []int

	legs           int   // to win the game
	legDarts       []int // per player, in the current leg
	starter        int   // the player who began the current leg
	waitForTakeout bool
	waiting        afterTakeout
	over           bool
	legWinner      *int     // who won a leg with the last dart, while the game goes on
	bull           *bullOff // nil without bulling off
}

// Advance counts a dart thrown; the turn is over after the third dart.
func (t *Turn) Advance() {
	t.count()
	if t.Dart == DartsPerTurn {
		t.pass(nextPlayer)
	}
}

// End counts a dart that ends the turn early, e.g. a bust.
func (t *Turn) End() {
	t.count()
	t.pass(nextPlayer)
}

// Finish counts the dart the current player finished with: they win the leg, and the
// game once they have won enough legs.
func (t *Turn) Finish() {
	t.count()
	if best := t.BestLeg[t.Player]; best == 0 || t.legDarts[t.Player] < best {
		t.BestLeg[t.Player] = t.legDarts[t.Player]
	}
	t.LegsWon[t.Player]++
	if t.LegsWon[t.Player] >= t.legs {
		t.over = true
		return
	}
	winner := t.Player
	t.legWinner = &winner
	t.pass(nextLeg)
}

// count counts a dart of the current player.
func (t *Turn) count() {
	t.Dart++
	t.legDarts[t.Player]++
}

// pass ends the turn: then comes the next player or leg, or, when waiting for takeouts,
// after the darts were pulled out.
func (t *Turn) pass(next afterTakeout) {
	if t.waitForTakeout {
		t.waiting = next
		return
	}
	t.do(next)
}

func (t *Turn) do(next afterTakeout) {
	switch next {
	case nextPlayer:
		t.Dart = 0
		t.Player = (t.Player + 1) % len(t.LegsWon)
		if t.Player == t.starter {
			t.Round++
		}
	case nextLeg:
		t.Leg, t.Round, t.Dart = t.Leg+1, 1, 0
		clear(t.legDarts)
		t.starter = (t.starter + 1) % len(t.LegsWon)
		t.Player = t.starter
		if t.OnLegStart != nil {
			t.OnLegStart()
		}
	}
}

// Takeout handles the darts being pulled out: a turn waiting for it, or one with fewer
// than three darts (the others bounced off), passes on.
func (t *Turn) Takeout() {
	switch {
	case t.waiting != nothing:
		next := t.waiting
		t.waiting = nothing
		t.do(next)
	case t.Dart > 0 && !t.over:
		t.do(nextPlayer)
	}
}

// Waiting reports whether the turn is over but the darts weren't pulled out yet.
func (t *Turn) Waiting() bool {
	return t.waiting != nothing
}

func (t *Turn) Over() bool {
	return t.over
}
