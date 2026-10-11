package turn

import (
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

func TestReplayWithoutTakeouts(t *testing.T) {
	// Games logged before takeouts were recorded: a dart while waiting counts as pulled out.
	log := Log{Darts: make([]throw.Dart, 4), WaitForTakeout: true}
	tr := NewWaitingForTakeouts()
	var players []int
	if err := log.Replay(&tr, 2, func(i int, d throw.Dart) error {
		players = append(players, tr.Player)
		tr.Advance(2)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if want := []int{0, 0, 0, 1}; len(players) != 4 || players[3] != want[3] {
		t.Errorf("players = %v, want %v", players, want)
	}

	// With a takeout logged after the third dart, the fourth dart is the next player's, too.
	log.Takeouts = []int{3}
	tr = NewWaitingForTakeouts()
	_ = log.Replay(&tr, 2, func(i int, d throw.Dart) error { tr.Advance(2); return nil })
	if tr.Player != 1 || tr.Dart != 1 {
		t.Errorf("player %d darts %d, want player 1 with 1 dart", tr.Player, tr.Dart)
	}
}
