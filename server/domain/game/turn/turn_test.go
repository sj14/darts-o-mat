package turn

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/common/throw"
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
	if tr.Over() || tr.Leg != 2 || tr.Player != 1 || tr.Round != 1 || tr.Records[0].Legs != 1 || legStarts != 1 {
		t.Fatalf("after leg 1: over=%v leg=%d player=%d records=%v, want leg 2 started by player 1", tr.Over(), tr.Leg, tr.Player, tr.Records)
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

func TestSets(t *testing.T) {
	tr := Log{Legs: 2, Sets: 2, WaitForTakeout: true}.Turn(2)
	check := func(step string, set, leg, player int) {
		t.Helper()
		if tr.Over() || tr.Set != set || tr.Leg != leg || tr.Player != player {
			t.Fatalf("%s: set %d leg %d player %d over %v, want set %d leg %d started by player %d", step, tr.Set, tr.Leg, tr.Player, tr.Over(), set, leg, player)
		}
	}
	finish := func() {
		tr.Finish()
		tr.Takeout()
	}

	finish() // player 0 wins leg 1
	check("after leg 1", 1, 2, 1)
	finish() // player 1 wins leg 2
	check("after leg 2", 1, 3, 0)
	tr.Finish() // player 0 wins leg 3 and the set
	if s := tr.State([]int64{10, 11}); !s.LegOver || !s.SetOver || s.SetWinner == nil || *s.SetWinner != 10 {
		t.Fatalf("after the set: leg over %v, set over %v, set winner %v, want set over, won by 10", s.LegOver, s.SetOver, s.SetWinner)
	}
	tr.Takeout()
	// The second set is started by the next player after the one who started the first.
	check("after set 1", 2, 1, 1)
	if r := tr.Records[0]; r.Sets != 1 || r.Legs != 2 || r.SetLegs != 0 {
		t.Fatalf("player 0 has %+v, want 1 set, 2 legs, none in the new set", r)
	}
	finish() // player 1 wins leg 1 of set 2
	check("set 2, after leg 1", 2, 2, 0)
	finish() // player 0
	check("set 2, after leg 2", 2, 3, 1)
	finish() // player 1 wins set 2
	check("after set 2", 3, 1, 0)
	finish() // player 0
	check("set 3, after leg 1", 3, 2, 1)
	for range DartsPerTurn {
		tr.Advance()
	}
	tr.Takeout()
	tr.Finish() // player 0 wins set 3 and the game
	if !tr.Over() || tr.Player != 0 || tr.Waiting() {
		t.Errorf("player %d over %v waiting %v, want player 0 to win at once", tr.Player, tr.Over(), tr.Waiting())
	}
}

func TestReplay(t *testing.T) {
	dart, takeout := throw.Dart{Number: 20, Segment: throw.Single}, throw.Dart{Takeout: true}
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
	at := func(x, y float64) throw.Dart { return throw.Dart{Number: 20, Segment: throw.Single, X: &x, Y: &y} }
	bullseye := throw.Dart{Number: throw.BullNumber, Segment: throw.Bullseye}
	bull := throw.Dart{Number: throw.BullNumber, Segment: throw.Bull}
	miss := throw.Dart{Segment: throw.Miss}
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
	bullseye := throw.Dart{Number: throw.BullNumber, Segment: throw.Bullseye}
	miss := throw.Dart{Segment: throw.Miss}
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
	bull := throw.Dart{Number: throw.BullNumber, Segment: throw.Bull}
	miss := throw.Dart{Segment: throw.Miss}
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

// Replay records when the game and each finished leg and set were played: from their
// first dart to their last, takeouts aside.
func TestReplayTimes(t *testing.T) {
	at := func(min int) time.Time { return time.Date(2026, 10, 16, 20, min, 0, 0, time.UTC) }
	advance, finish := throw.Dart{Number: 1}, throw.Dart{Number: 2}
	entry := func(d throw.Dart, min int) throw.Dart { d.CreatedAt = at(min); return d }
	log := Log{Legs: 2, Sets: 2, Entries: []throw.Dart{
		entry(advance, 0), entry(finish, 1), // set 1, leg 1
		entry(finish, 3),                                                       // leg 2
		entry(advance, 5), {Takeout: true, CreatedAt: at(6)}, entry(finish, 8), // set 2, leg 1
		entry(finish, 9), // leg 2 and the game
		{Takeout: true, CreatedAt: at(10)},
	}}
	tr := log.Turn(1)
	err := log.Replay(tr, func(_ int, d throw.Dart) error {
		if d.Number == finish.Number {
			tr.Finish()
		} else {
			tr.Advance()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	st := tr.State([]int64{7})
	span := func(from, to int) Span { return Span{at(from), at(to)} }
	wantLegs := []LegTime{{1, 1, span(0, 1)}, {1, 2, span(3, 3)}, {2, 1, span(5, 8)}, {2, 2, span(9, 9)}}
	if !slices.Equal(st.Times.Legs, wantLegs) {
		t.Errorf("legs %v, want %v", st.Times.Legs, wantLegs)
	}
	if want := []Span{span(0, 3), span(5, 9)}; !slices.Equal(st.Times.Sets, want) {
		t.Errorf("sets %v, want %v", st.Times.Sets, want)
	}
	if st.Times.Game != span(0, 9) || st.Times.Game.Duration() != 9*time.Minute {
		t.Errorf("game %v, want from minute 0 to 9", st.Times.Game)
	}
	if got := st.Times.Leg(2, 1).Duration(); got != 3*time.Minute {
		t.Errorf("leg 2 of set 2 took %v, want 3m", got)
	}
}

func TestLeave(t *testing.T) {
	// The player who is up leaves: their turn is over, the next one throws.
	tr := Log{}.Turn(3)
	tr.Advance()
	if err := tr.Leave(0); err != nil {
		t.Fatal(err)
	}
	if tr.Player != 1 || tr.Dart != 0 || tr.Round != 1 || !tr.Records[0].Left {
		t.Errorf("after the player up left: player %d dart %d round %d, want player 1 in round 1", tr.Player, tr.Dart, tr.Round)
	}
	// Players who left are skipped, a round is over when the new starter is up again.
	for range DartsPerTurn {
		tr.Advance()
	}
	for range DartsPerTurn {
		tr.Advance()
	}
	if tr.Player != 1 || tr.Round != 2 {
		t.Errorf("after a lap: player %d round %d, want player 1 in round 2", tr.Player, tr.Round)
	}
	// With two players left the game can't go on without one of them.
	if err := tr.Leave(1); !errors.Is(err, ErrCantLeave) {
		t.Errorf("leaving with two players: got %v, want ErrCantLeave", err)
	}

	// A player who isn't up leaves; the starter of the leg leaves and the next one starts.
	tr = Log{Legs: 2}.Turn(4)
	if err := tr.Leave(2); err != nil {
		t.Fatal(err)
	}
	for range DartsPerTurn {
		tr.Advance()
	} // 0
	for range DartsPerTurn {
		tr.Advance()
	} // 1
	if tr.Player != 3 {
		t.Errorf("after player 1: player %d, want 3 (2 left)", tr.Player)
	}
	if err := tr.Leave(0); err != nil {
		t.Fatal(err)
	}
	tr.Finish() // player 3 wins leg 1
	if tr.Leg != 2 || tr.Player != 1 {
		t.Errorf("leg %d started by player %d, want leg 2 by player 1 (0 left, 1 is next)", tr.Leg, tr.Player)
	}
}

func TestLeaveWhileWaitingForTakeout(t *testing.T) {
	tr := Log{WaitForTakeout: true}.Turn(3)
	for range DartsPerTurn {
		tr.Advance()
	}
	if err := tr.Leave(0); err != nil {
		t.Fatal(err)
	}
	if !tr.Waiting() || tr.Player != 0 {
		t.Fatalf("waiting %v player %d, want still waiting for the takeout of the player who left", tr.Waiting(), tr.Player)
	}
	tr.Takeout()
	if tr.Player != 1 || tr.Waiting() {
		t.Errorf("after the takeout: player %d, want 1", tr.Player)
	}
}

func TestReplayLeaveAndThrowers(t *testing.T) {
	ids := []int64{10, 20, 30}
	dart := func(id int64) throw.Dart { return throw.Dart{PlayerID: id, Number: 1, Segment: throw.Single} }
	log := Log{PlayerIDs: ids, Entries: []throw.Dart{dart(10), {PlayerID: 10, Leave: true}, dart(20)}}
	tr := log.Turn(3)
	if err := log.Replay(tr, func(int, throw.Dart) error { tr.Advance(); return nil }); err != nil {
		t.Fatal(err)
	}
	if !tr.Records[0].Left || tr.Player != 1 || tr.Dart != 1 || !slices.Equal(tr.Throwers, []int{0, -1, 1}) {
		t.Errorf("player %d dart %d throwers %v left %v, want player 1 with 1 dart", tr.Player, tr.Dart, tr.Throwers, tr.Records[0].Left)
	}

	// A dart recorded for another player than the one who is up is refused.
	log = Log{PlayerIDs: ids, Entries: []throw.Dart{dart(20)}}
	tr = log.Turn(3)
	if err := log.Replay(tr, func(int, throw.Dart) error { return nil }); !errors.Is(err, ErrWrongThrower) {
		t.Errorf("a dart of player 20 in the turn of 10: got %v, want ErrWrongThrower", err)
	}
	// Leaving a game the player isn't in, or the last two players leaving, is refused.
	log = Log{PlayerIDs: ids, Entries: []throw.Dart{{PlayerID: 99, Leave: true}}}
	if err := log.Replay(log.Turn(3), nil); !errors.Is(err, ErrCantLeave) {
		t.Errorf("a stranger leaves: got %v, want ErrCantLeave", err)
	}
}

func TestSkip(t *testing.T) {
	// A player who took too long is passed over, with the darts they threw.
	tr := Log{}.Turn(3)
	tr.Advance()
	tr.Skip()
	if tr.Player != 1 || tr.Dart != 0 {
		t.Errorf("after a skipped turn: player %d dart %d, want player 1 without darts", tr.Player, tr.Dart)
	}
	// Nothing to skip while the turn waits for the takeout, or while bulling off.
	tr = Log{WaitForTakeout: true}.Turn(2)
	for range DartsPerTurn {
		tr.Advance()
	}
	tr.Skip()
	if !tr.Waiting() || tr.Player != 0 {
		t.Errorf("skipped while waiting for the takeout: player %d, waiting %v", tr.Player, tr.Waiting())
	}
	tr = Log{BullOff: true}.Turn(2)
	tr.Skip()
	if tr.Player != 0 {
		t.Errorf("skipped while bulling off: player %d, want 0", tr.Player)
	}
}

func TestReplaySkip(t *testing.T) {
	ids := []int64{10, 20, 30}
	dart := func(id int64) throw.Dart { return throw.Dart{PlayerID: id, Number: 1, Segment: throw.Single} }
	log := Log{PlayerIDs: ids, Entries: []throw.Dart{dart(10), {PlayerID: 10, Skip: true}, {PlayerID: 20, Skip: true}, dart(30)}}
	tr := log.Turn(3)
	if err := log.Replay(tr, func(int, throw.Dart) error { tr.Advance(); return nil }); err != nil {
		t.Fatal(err)
	}
	if tr.Player != 2 || tr.Dart != 1 || !slices.Equal(tr.Throwers, []int{0, -1, -1, 2}) {
		t.Errorf("player %d dart %d throwers %v, want player 2 with 1 dart", tr.Player, tr.Dart, tr.Throwers)
	}
	// A skip for another player than the one who is up is refused.
	log = Log{PlayerIDs: ids, Entries: []throw.Dart{{PlayerID: 20, Skip: true}}}
	if err := log.Replay(log.Turn(3), nil); !errors.Is(err, ErrWrongThrower) {
		t.Errorf("a skip of 20 in the turn of 10: got %v, want ErrWrongThrower", err)
	}
}
