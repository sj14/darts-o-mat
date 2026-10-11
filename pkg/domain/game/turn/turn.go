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
	t := &Turn{Round: 1, Leg: 1, LegsWon: make([]int, players), players: players, legs: max(l.Legs, 1), waitForTakeout: l.WaitForTakeout, winner: -1}
	if l.BullOff && players > 1 {
		for i := range players {
			t.contenders = append(t.contenders, i)
		}
		t.closer = l.BullOffCloser
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
			t.bullOff(i, d)
			continue
		}
		if err := dart(i, d); err != nil {
			return err
		}
	}
	return nil
}

type Turn struct {
	Player  int   // index of the player whose turn it is
	Round   int   // of the current leg, starts at 1
	Dart    int   // darts already thrown in this turn
	Leg     int   // starts at 1
	LegsWon []int // per player
	// OnLegStart is called when the next leg starts, so the game mode resets the scores.
	OnLegStart func()
	// Throwers is the index of the player who threw each log entry, -1 for takeouts.
	Throwers []int
	// BullOff lists the darts thrown to bull off, in order.
	BullOff []BullOffThrow

	contenders     []int // players still bulling off
	closer         *int  // see Log.BullOffCloser
	bullOffRound   int   // 0 for the first throw of everyone, then the ties
	first          int   // the player who starts the first leg
	players, legs  int
	waitForTakeout bool
	waiting        bool
	legOver        bool // the turn waiting for the takeout won a leg
	winner         int  // -1 until the game is over
}

// Advance counts a dart thrown; the turn is over after the third dart.
func (t *Turn) Advance() {
	t.Dart++
	if t.Dart == DartsPerTurn {
		t.complete()
	}
}

// End counts a dart that ends the turn early, e.g. a bust.
func (t *Turn) End() {
	t.Dart++
	t.complete()
}

// Finish counts the dart the current player finished with: they win the leg, and the
// game once they have won enough legs.
func (t *Turn) Finish() {
	t.Dart++
	t.LegsWon[t.Player]++
	if t.LegsWon[t.Player] >= t.legs {
		t.winner = t.Player
		return
	}
	t.legOver = true
	if t.waitForTakeout {
		t.waiting = true
		return
	}
	t.nextLeg()
}

// complete ends the turn: the next player throws, or, when waiting for takeouts,
// after the darts were pulled out.
func (t *Turn) complete() {
	if t.waitForTakeout {
		t.waiting = true
		return
	}
	t.next()
}

// Takeout handles the darts being pulled out: a turn waiting for it, or one with fewer
// than three darts (the others bounced off), passes on.
func (t *Turn) Takeout() {
	switch {
	case t.waiting && t.legOver:
		t.waiting = false
		t.nextLeg()
	case t.waiting || (t.Dart > 0 && !t.Over()):
		t.waiting = false
		t.next()
	}
}

// Waiting reports whether the turn is over but the darts weren't pulled out yet.
func (t *Turn) Waiting() bool {
	return t.waiting
}

// LegOver reports whether the turn waiting for the takeout won a leg.
func (t *Turn) LegOver() bool {
	return t.legOver
}

func (t *Turn) Over() bool {
	return t.winner >= 0
}

// Winner returns the index of the player who won the game, false until it is over.
func (t *Turn) Winner() (int, bool) {
	return t.winner, t.winner >= 0
}

// starter is the player who began the current leg.
func (t *Turn) starter() int {
	return (t.first + t.Leg - 1) % t.players
}

func (t *Turn) next() {
	t.Dart = 0
	t.Player = (t.Player + 1) % t.players
	if t.Player == t.starter() {
		t.Round++
	}
}

func (t *Turn) nextLeg() {
	t.Leg++
	t.Round, t.Dart, t.legOver = 1, 0, false
	t.Player = t.starter()
	if t.OnLegStart != nil {
		t.OnLegStart()
	}
}
