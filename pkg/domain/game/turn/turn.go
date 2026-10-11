// Package turn contains the turn order shared by all game modes:
// the players throw three darts each, one after another.
//
// Once a player has finished, the remaining players of that round may still
// throw their turn. The game is over at the end of the round and everyone who
// finished in it wins.
//
// Games played on a board with cameras wait after each turn until the darts were
// pulled out (a takeout), so the next player doesn't start while the darts are
// still in the board.
package turn

import (
	"errors"

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
	Darts []throw.Dart
	// Takeouts are the numbers of darts thrown before each takeout, ascending.
	Takeouts []int
	// WaitForTakeout keeps a finished turn until the darts were pulled out.
	WaitForTakeout bool
}

// Replay calls dart for every dart of the log in order and handles the takeouts in
// between. A dart while waiting for a takeout counts as if the darts had been pulled
// out (games logged before takeouts were recorded); rejecting such darts is up to the
// caller, see Turn.Waiting.
func (l Log) Replay(t *Turn, players int, dart func(i int, d throw.Dart) error) error {
	next := 0
	takeouts := func(thrown int) {
		for next < len(l.Takeouts) && l.Takeouts[next] <= thrown {
			t.Takeout(players)
			next++
		}
	}
	for i, d := range l.Darts {
		takeouts(i)
		if t.Waiting() {
			t.Takeout(players)
		}
		if err := dart(i, d); err != nil {
			return err
		}
	}
	takeouts(len(l.Darts))
	return nil
}

type Turn struct {
	Player int // index of the player whose turn it is
	Round  int // starts at 1
	Dart   int // darts already thrown in this turn

	waitForTakeout bool
	waiting        bool
	finished       []int
	over           bool
}

func New() Turn {
	return Turn{Round: 1}
}

// NewWaitingForTakeouts returns a turn order that keeps each finished turn until Takeout.
func NewWaitingForTakeouts() Turn {
	return Turn{Round: 1, waitForTakeout: true}
}

// Advance moves on after a dart was thrown; the turn is over after the third dart.
func (t *Turn) Advance(players int) {
	t.Dart++
	if t.Dart == DartsPerTurn {
		t.End(players)
	}
}

// End ends the turn, e.g. after the third dart or a bust: the next player throws,
// or, when waiting for takeouts, after the darts were pulled out.
func (t *Turn) End(players int) {
	if t.waitForTakeout && !t.endsGame(players) {
		t.waiting = true
		return
	}
	t.next(players)
}

// Finish marks the current player as finished and ends their turn.
func (t *Turn) Finish(players int) {
	t.finished = append(t.finished, t.Player)
	t.End(players)
}

// Takeout handles the darts being pulled out: a turn waiting for it, or one with fewer
// than three darts (the others bounced off), passes to the next player.
func (t *Turn) Takeout(players int) {
	if t.waiting || t.Dart > 0 {
		t.waiting = false
		t.next(players)
	}
}

// Waiting reports whether the turn is over but the darts weren't pulled out yet.
func (t *Turn) Waiting() bool {
	return t.waiting
}

func (t *Turn) Over() bool {
	return t.over
}

// Winners returns the indices of the players who finished, empty until the game is over.
func (t *Turn) Winners() []int {
	if !t.over {
		return nil
	}
	return t.finished
}

// endsGame reports whether passing the turn on now ends the game: the round is complete
// and someone finished. There's no need to wait for the takeout then.
func (t *Turn) endsGame(players int) bool {
	return (t.Player+1)%players == 0 && len(t.finished) > 0
}

func (t *Turn) next(players int) {
	t.Dart = 0
	t.Player = (t.Player + 1) % players
	if t.Player != 0 {
		return
	}
	if len(t.finished) > 0 {
		t.over = true
		return
	}
	t.Round++
}
