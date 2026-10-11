package turn

import (
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func TestWaitForTakeout(t *testing.T) {
	tr := NewWaitingForTakeouts()
	const players = 2

	for range DartsPerTurn {
		tr.Advance(players)
	}
	if !tr.Waiting() || tr.Player != 0 || tr.Dart != DartsPerTurn {
		t.Fatalf("after three darts: waiting=%v player=%d darts=%d, want player 0 waiting with 3 darts", tr.Waiting(), tr.Player, tr.Dart)
	}
	tr.Takeout(players)
	if tr.Waiting() || tr.Player != 1 || tr.Dart != 0 {
		t.Fatalf("after takeout: waiting=%v player=%d, want player 1", tr.Waiting(), tr.Player)
	}

	// Pulling the darts out early (some bounced off) ends the turn, too.
	tr.Advance(players)
	tr.Takeout(players)
	if tr.Player != 0 || tr.Round != 2 {
		t.Errorf("after an early takeout: player %d round %d, want player 0 in round 2", tr.Player, tr.Round)
	}

	// A takeout without darts thrown changes nothing.
	tr.Takeout(players)
	if tr.Player != 0 {
		t.Errorf("empty takeout passed the turn on")
	}
}

func TestWaitForTakeoutGameOver(t *testing.T) {
	// The last player of the round checks out: the game is over without waiting.
	tr := NewWaitingForTakeouts()
	tr.Player = 1
	tr.Finish(2)
	if tr.Waiting() || !tr.Over() {
		t.Errorf("waiting=%v over=%v, want the game over", tr.Waiting(), tr.Over())
	}
}

func TestReplay(t *testing.T) {
	dart, takeout := throw.Dart{Number: 20, Multiplier: 1}, throw.Dart{Takeout: true}
	advance := func(tr *Turn) func(int, throw.Dart) error {
		return func(int, throw.Dart) error { tr.Advance(2); return nil }
	}

	// The takeout after the third dart passes the turn on to the next player.
	tr := NewWaitingForTakeouts()
	log := Log{Entries: []throw.Dart{dart, dart, dart, takeout, dart}, WaitForTakeout: true}
	if err := log.Replay(&tr, 2, advance(&tr)); err != nil {
		t.Fatal(err)
	}
	if tr.Player != 1 || tr.Dart != 1 {
		t.Errorf("player %d with %d darts, want player 1 with 1 dart", tr.Player, tr.Dart)
	}

	// A dart before the takeout is an error.
	tr = NewWaitingForTakeouts()
	log.Entries = []throw.Dart{dart, dart, dart, dart}
	if err := log.Replay(&tr, 2, advance(&tr)); !errors.Is(err, ErrWaitingForTakeout) {
		t.Errorf("got %v, want ErrWaitingForTakeout", err)
	}
}
