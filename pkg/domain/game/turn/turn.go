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
}

// Replay calls dart for every dart of the log in order and handles the takeouts in
// between. A dart while the turn waits for the takeout is an error.
func (l Log) Replay(t *Turn, players int, dart func(i int, d throw.Dart) error) error {
	for i, d := range l.Entries {
		if d.Takeout {
			t.Takeout(players)
			continue
		}
		if t.Waiting() {
			return fmt.Errorf("entry %d: %w", i+1, ErrWaitingForTakeout)
		}
		if err := dart(i, d); err != nil {
			return err
		}
	}
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

// Advance counts a dart thrown; the turn is over after the third dart.
func (t *Turn) Advance(players int) {
	t.Dart++
	if t.Dart == DartsPerTurn {
		t.complete(players)
	}
}

// End counts a dart that ends the turn early, e.g. a bust.
func (t *Turn) End(players int) {
	t.Dart++
	t.complete(players)
}

// Finish counts the dart the current player finished with and ends their turn.
func (t *Turn) Finish(players int) {
	t.finished = append(t.finished, t.Player)
	t.End(players)
}

// complete ends the turn: the next player throws, or, when waiting for takeouts,
// after the darts were pulled out.
func (t *Turn) complete(players int) {
	if t.waitForTakeout && !t.endsGame(players) {
		t.waiting = true
		return
	}
	t.next(players)
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
