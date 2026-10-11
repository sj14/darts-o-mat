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
	if !tr.Over() || tr.Player != 0 || tr.Waiting() {
		t.Errorf("player=%d over=%v waiting=%v, want player 0 to win at once", tr.Player, tr.Over(), tr.Waiting())
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
	if !tr.Over() || tr.Player != 0 {
		t.Errorf("player %d over %v, want player 0 to win", tr.Player, tr.Over())
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

func TestBullOff(t *testing.T) {
	at := func(x, y float64) throw.Dart { return throw.Dart{Number: 20, Multiplier: 1, X: &x, Y: &y} }
	bullseye := throw.Dart{Number: throw.Bull, Multiplier: 2}
	bull := throw.Dart{Number: throw.Bull, Multiplier: 1}
	miss := throw.Dart{Miss: true}
	var played []int
	replay := func(entries ...throw.Dart) *Turn {
		t.Helper()
		played = nil
		log := Log{BullOff: true, Entries: entries}
		tr := log.Turn(3)
		if err := log.Replay(tr, func(i int, _ throw.Dart) error { played = append(played, i); tr.Advance(); return nil }); err != nil {
			t.Fatal(err)
		}
		return tr
	}

	// Positions from the cameras: player 2 is closest and starts.
	tr := replay(at(30, 0), at(0, 40), at(5, 5), throw.Dart{Takeout: true}, bull)
	if tr.BullingOff() || tr.Throwers[3] != -1 || len(played) != 1 || played[0] != 4 || tr.Throwers[4] != 2 {
		t.Fatalf("bulling off=%v played=%v throwers=%v, want player 2 to start with entry 4", tr.BullingOff(), played, tr.Throwers)
	}
	// Entered by hand: the bullseye beats the bull, which beats the rest.
	tr = replay(bull, miss, bullseye)
	if tr.BullingOff() || tr.Player != 2 {
		t.Errorf("player %d, want player 2 with the bullseye to start", tr.Player)
	}

	// A tie: only the tied players throw again.
	tr = replay(bull, miss, bull)
	if !tr.BullingOff() || tr.Player != 0 {
		t.Fatalf("bulling off=%v player=%d, want players 0 and 2 to throw again, 0 first", tr.BullingOff(), tr.Player)
	}
	tr = replay(bull, miss, bull, miss, bullseye)
	if tr.BullingOff() || tr.Player != 2 || len(played) != 0 {
		t.Errorf("bulling off=%v player=%d, want player 2 to win the tie-break", tr.BullingOff(), tr.Player)
	}
}

func TestBullOffStartsLegs(t *testing.T) {
	bullseye := throw.Dart{Number: throw.Bull, Multiplier: 2}
	miss := throw.Dart{Miss: true}
	log := Log{BullOff: true, Legs: 2, Entries: []throw.Dart{miss, bullseye}}
	tr := log.Turn(2)
	if err := log.Replay(tr, func(int, throw.Dart) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if tr.Player != 1 {
		t.Fatalf("player %d starts leg 1, want player 1", tr.Player)
	}
	tr.Finish() // player 1 wins leg 1, leg 2 is started by the next player
	if tr.Leg != 2 || tr.Player != 0 {
		t.Errorf("leg %d started by player %d, want leg 2 by player 0", tr.Leg, tr.Player)
	}
}

func TestBullOffCloser(t *testing.T) {
	bull := throw.Dart{Number: throw.Bull, Multiplier: 1}
	miss := throw.Dart{Miss: true}
	two := 2
	log := Log{BullOff: true, Entries: []throw.Dart{bull, miss, bull}}
	tr := log.Turn(3)
	if err := log.Replay(tr, func(int, throw.Dart) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if got := tr.State([]int64{10, 11, 12}).BullOffTie; len(got) != 2 || got[0] != 10 || got[1] != 12 {
		t.Fatalf("tie = %v, want players 10 and 12", got)
	}

	// The players decide that player 2 was closer: they start without throwing again.
	log.BullOffCloser = &two
	tr = log.Turn(3)
	if err := log.Replay(tr, func(int, throw.Dart) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if tr.BullingOff() || tr.Player != 2 || tr.State([]int64{10, 11, 12}).BullOffTie != nil {
		t.Errorf("bulling off=%v player=%d, want player 2 to start", tr.BullingOff(), tr.Player)
	}
}
