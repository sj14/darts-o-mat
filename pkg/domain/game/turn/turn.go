// Package turn contains the turn order shared by all game modes:
// the players throw three darts each, one after another.
//
// Once a player has finished, the remaining players of that round may still
// throw their turn. The game is over at the end of the round and everyone who
// finished in it wins.
package turn

import "errors"

const DartsPerTurn = 3

var ErrGameOver = errors.New("game is already over")

type Turn struct {
	Player int // index of the player whose turn it is
	Round  int // starts at 1
	Dart   int // darts already thrown in this turn

	finished []int
	over     bool
}

func New() Turn {
	return Turn{Round: 1}
}

// Advance moves on after a dart was thrown and ends the turn after the third dart.
func (t *Turn) Advance(players int) {
	t.Dart++
	if t.Dart == DartsPerTurn {
		t.End(players)
	}
}

// End passes the turn to the next player, e.g. after the third dart or a bust.
func (t *Turn) End(players int) {
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

// Finish marks the current player as finished and ends their turn.
func (t *Turn) Finish(players int) {
	t.finished = append(t.finished, t.Player)
	t.End(players)
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
