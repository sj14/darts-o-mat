package turn

import (
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func TestWaitForTakeout(t *testing.T) {
	tr := Log{WaitForTakeout: true}.Turn(2)

	for range DartsPerTurn {
		tr.Advance()
	}
	if !tr.Waiting() || tr.Player != 0 || tr.Dart != DartsPerTurn {
		t.Fatalf("after three darts: waiting=%v player=%d darts=%d, want player 0 waiting with 3 darts", tr.Waiting(), tr.Player, tr.Dart)
	}
	tr.Takeout()
	if tr.Waiting() || tr.Player != 1 || tr.Dart != 0 {
		t.Fatalf("after takeout: waiting=%v player=%d, want player 1", tr.Waiting(), tr.Player)
	}

	// Pulling the darts out early (some bounced off) ends the turn, too.
	tr.Advance()
	tr.Takeout()
	if tr.Player != 0 || tr.Round != 2 {
		t.Errorf("after an early takeout: player %d round %d, want player 0 in round 2", tr.Player, tr.Round)
	}

	// A takeout without darts thrown changes nothing.
	tr.Takeout()
	if tr.Player != 0 {
		t.Errorf("empty takeout passed the turn on")
	}
}

func TestFinish(t *testing.T) {
	// The checkout wins at once, also on a board: there is nothing to wait for.
	tr := Log{WaitForTakeout: true}.Turn(2)
	tr.Finish()
	if w, over := tr.Winner(); !over || w != 0 || tr.Waiting() {
		t.Errorf("winner=%d over=%v waiting=%v, want player 0 to win at once", w, over, tr.Waiting())
	}
}

func TestLegs(t *testing.T) {
	legStarts := 0
	tr := Log{Legs: 2}.Turn(3)
	tr.OnLegStart = func() { legStarts++ }

	tr.Advance()
	tr.Finish() // player 0 wins leg 1
	if tr.Over() || tr.Leg != 2 || tr.Player != 1 || tr.Round != 1 || tr.LegsWon[0] != 1 || legStarts != 1 {
		t.Fatalf("after leg 1: over=%v leg=%d player=%d legs=%v, want leg 2 started by player 1", tr.Over(), tr.Leg, tr.Player, tr.LegsWon)
	}
	// A round of leg 2 is over when its starter throws again.
	for range 3 * DartsPerTurn {
		tr.Advance()
	}
	if tr.Player != 1 || tr.Round != 2 {
		t.Errorf("player %d round %d, want player 1 in round 2", tr.Player, tr.Round)
	}
	tr.Finish() // player 1 wins leg 2
	tr.Finish() // player 2 starts leg 3 and wins it
	if tr.Over() || tr.Leg != 4 || tr.Player != 0 {
		t.Fatalf("leg %d player %d, want leg 4 started by player 0", tr.Leg, tr.Player)
	}
	tr.Finish() // player 0 wins a second leg and the game
	if w, over := tr.Winner(); !over || w != 0 {
		t.Errorf("winner %d over %v, want player 0", w, over)
	}
}

func TestReplay(t *testing.T) {
	dart, takeout := throw.Dart{Number: 20, Multiplier: 1}, throw.Dart{Takeout: true}
	advance := func(tr *Turn) func(int, throw.Dart) error {
		return func(int, throw.Dart) error { tr.Advance(); return nil }
	}

	// The takeout after the third dart passes the turn on to the next player.
	log := Log{Entries: []throw.Dart{dart, dart, dart, takeout, dart}, WaitForTakeout: true}
	tr := log.Turn(2)
	if err := log.Replay(tr, advance(tr)); err != nil {
		t.Fatal(err)
	}
	if tr.Player != 1 || tr.Dart != 1 {
		t.Errorf("player %d with %d darts, want player 1 with 1 dart", tr.Player, tr.Dart)
	}

	// A dart before the takeout is an error.
	log.Entries = []throw.Dart{dart, dart, dart, dart}
	tr = log.Turn(2)
	if err := log.Replay(tr, advance(tr)); !errors.Is(err, ErrWaitingForTakeout) {
		t.Errorf("got %v, want ErrWaitingForTakeout", err)
	}
}
