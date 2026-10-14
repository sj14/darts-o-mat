package x01

import (
	"errors"
	"reflect"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func hit(number, multiplier uint8) throw.Dart {
	return throw.Dart{Number: number, Multiplier: multiplier}
}

var miss = throw.Dart{Miss: true}

func TestPlay(t *testing.T) {
	tests := []struct {
		name          string
		settings      Settings
		darts         []throw.Dart
		wantRemaining []uint64
		wantCurrent   int64 // 0 means the game is over
		wantRound     int
		wantDarts     int
		wantWinner    int64 // 0 while the game goes on
	}{
		{
			name:          "no darts",
			settings:      Settings{StartScore: 501, OutMode: OutModeDouble},
			wantRemaining: []uint64{501, 501},
			wantCurrent:   1,
			wantRound:     1,
		},
		{
			name:          "turn passes after three darts",
			settings:      Settings{StartScore: 501, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 3), hit(20, 3), hit(20, 3), hit(5, 1)},
			wantRemaining: []uint64{321, 496},
			wantCurrent:   2,
			wantRound:     1,
			wantDarts:     1,
		},
		{
			name:          "next round after all players",
			settings:      Settings{StartScore: 501, OutMode: OutModeDouble},
			darts:         []throw.Dart{miss, miss, miss, miss, miss, miss},
			wantRemaining: []uint64{501, 501},
			wantCurrent:   1,
			wantRound:     2,
		},
		{
			name:          "overshoot busts and resets the whole turn",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(10, 1), hit(20, 2)},
			wantRemaining: []uint64{40, 40},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "leaving 1 on double out busts",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(13, 3)},
			wantRemaining: []uint64{40, 40},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "finishing on a single busts on double out",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 1), hit(20, 1)},
			wantRemaining: []uint64{40, 40},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "the checkout wins at once, the others don't throw anymore",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 1), hit(10, 2)},
			wantRemaining: []uint64{0, 40},
			wantRound:     1,
			wantDarts:     2,
			wantWinner:    1,
		},
		{
			name:          "bullseye counts as double",
			settings:      Settings{StartScore: 50, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(throw.Bull, 2)},
			wantRemaining: []uint64{0, 50},
			wantRound:     1,
			wantDarts:     1,
			wantWinner:    1,
		},
		{
			name:          "checkout by the second player",
			settings:      Settings{StartScore: 20, OutMode: OutModeSingle},
			darts:         []throw.Dart{miss, miss, miss, hit(19, 1), hit(1, 1)},
			wantRemaining: []uint64{20, 0},
			wantRound:     1,
			wantDarts:     2,
			wantWinner:    2,
		},
		{
			name:          "master out finishes on a double",
			settings:      Settings{StartScore: 6, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(3, 2)},
			wantRemaining: []uint64{0, 6},
			wantRound:     1,
			wantDarts:     1,
			wantWinner:    1,
		},
		{
			name:          "master out finishes on a triple",
			settings:      Settings{StartScore: 12, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(3, 2), hit(2, 3)},
			wantRemaining: []uint64{0, 12},
			wantRound:     1,
			wantDarts:     2,
			wantWinner:    1,
		},
		{
			name:          "finishing on a single busts on master out",
			settings:      Settings{StartScore: 6, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(6, 1)},
			wantRemaining: []uint64{6, 6},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "leaving 1 on master out busts",
			settings:      Settings{StartScore: 40, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(13, 3)},
			wantRemaining: []uint64{40, 40},
			wantCurrent:   2,
			wantRound:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := Play(tt.settings, []int64{1, 2}, tt.darts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for i, want := range tt.wantRemaining {
				if got := st.Players[i].Remaining; got != want {
					t.Errorf("player %d remaining = %d, want %d", i+1, got, want)
				}
			}
			var current int64
			if st.CurrentPlayerID != nil {
				current = *st.CurrentPlayerID
			}
			if current != tt.wantCurrent {
				t.Errorf("current player = %d, want %d", current, tt.wantCurrent)
			}
			if st.Round != tt.wantRound {
				t.Errorf("round = %d, want %d", st.Round, tt.wantRound)
			}
			if st.DartsInTurn != tt.wantDarts {
				t.Errorf("darts in turn = %d, want %d", st.DartsInTurn, tt.wantDarts)
			}
			var winner int64
			if st.WinnerID != nil {
				winner = *st.WinnerID
			}
			if winner != tt.wantWinner || st.GameOver != (tt.wantWinner != 0) {
				t.Errorf("winner = %d (game over %v), want %d", winner, st.GameOver, tt.wantWinner)
			}
		})
	}
}

func TestPlayErrors(t *testing.T) {
	s := Settings{StartScore: 40, OutMode: OutModeDouble}

	_, err := Play(s, []int64{1}, []throw.Dart{hit(20, 2), miss})
	if !errors.Is(err, turn.ErrGameOver) {
		t.Errorf("dart after win: got %v, want ErrGameOver", err)
	}

	_, err = Play(s, []int64{1}, []throw.Dart{hit(throw.Bull, 3)})
	if !errors.Is(err, throw.ErrInvalid) {
		t.Errorf("triple bull: got %v, want ErrInvalid", err)
	}
}

func TestOutModeJSON(t *testing.T) {
	var m OutMode
	if err := m.UnmarshalText([]byte("master")); err != nil || m != OutModeMaster {
		t.Errorf("unmarshal master: got %v, %v", m, err)
	}
	if err := m.UnmarshalText([]byte("triple")); err == nil {
		t.Error("unmarshal triple: want error")
	}
}

func TestStats(t *testing.T) {
	s := Settings{StartScore: 501, OutMode: OutModeDouble}
	darts := []throw.Dart{
		hit(20, 3), hit(20, 3), hit(20, 3), // player 1: 180
		hit(20, 1), hit(5, 1), hit(1, 1), // player 2: 26
		hit(20, 3), hit(20, 3), hit(19, 1), // player 1: 139
		miss, // player 2: open turn
	}
	st, err := Play(s, []int64{1, 2}, darts)
	if err != nil {
		t.Fatal(err)
	}

	p1 := st.Players[0].Stats
	if p1.Darts != 6 || p1.Average != 159.5 || p1.HighestTurn != 180 || p1.Turns180 != 1 || p1.Turns100 != 1 || p1.Turns140 != 0 {
		t.Errorf("player 1 stats = %+v", p1)
	}
	// The open turn counts its darts for the average, but not as a turn yet.
	p2 := st.Players[1].Stats
	if p2.Darts != 4 || p2.Average != 19.5 || p2.HighestTurn != 26 {
		t.Errorf("player 2 stats = %+v", p2)
	}
}

func TestStatsBustAndCheckout(t *testing.T) {
	s := Settings{StartScore: 100, OutMode: OutModeDouble}
	darts := []throw.Dart{
		hit(20, 3), hit(19, 1), hit(20, 1), // 21 left, then 20 leaves 1: bust, the turn scores 0
		hit(20, 3), hit(20, 2), // 100 checkout
	}
	st, err := Play(s, []int64{1}, darts)
	if err != nil {
		t.Fatal(err)
	}
	p := st.Players[0].Stats
	if p.Darts != 5 || p.Checkout != 100 || p.HighestTurn != 100 || p.Turns100 != 1 || p.Average != 60 {
		t.Errorf("stats = %+v", p)
	}
}

func TestTurns(t *testing.T) {
	s := Settings{StartScore: 100, OutMode: OutModeDouble}
	darts := []throw.Dart{
		hit(20, 3), hit(5, 1), hit(1, 1), // player 1 scores 66, 34 left
		hit(20, 1), hit(20, 3), hit(10, 1), // player 2 scores 90, 10 left
		hit(20, 1), hit(10, 1), hit(5, 1), // player 1: 14, 4, then the 5 busts
		hit(5, 2), // player 2 checks out
	}
	st, err := Play(s, []int64{1, 2}, darts)
	if err != nil {
		t.Fatal(err)
	}
	want := []Turn{
		{PlayerID: 1, Set: 1, Leg: 1, Round: 1, Points: 66, Remaining: 34, Complete: true},
		{PlayerID: 2, Set: 1, Leg: 1, Round: 1, Points: 90, Remaining: 10, Complete: true},
		{PlayerID: 1, Set: 1, Leg: 1, Round: 2, Points: 0, Remaining: 34, Bust: true, Complete: true},
		{PlayerID: 2, Set: 1, Leg: 1, Round: 2, Points: 10, Remaining: 0, Checkout: true, Complete: true},
	}
	wantDarts := []int{3, 3, 3, 1}
	if len(st.Turns) != len(want) {
		t.Fatalf("got %d turns, want %d: %+v", len(st.Turns), len(want), st.Turns)
	}
	for i, w := range want {
		got := st.Turns[i]
		if len(got.Darts) != wantDarts[i] {
			t.Errorf("turn %d has %d darts, want %d", i, len(got.Darts), wantDarts[i])
		}
		got.Darts, w.Darts = nil, nil
		if !reflect.DeepEqual(got, w) {
			t.Errorf("turn %d = %+v, want %+v", i, got, w)
		}
	}

	// A turn in progress.
	st, _ = Play(s, []int64{1, 2}, darts[:4])
	last := st.Turns[len(st.Turns)-1]
	if last.Complete || last.Points != 20 || last.Remaining != 80 || len(last.Darts) != 1 {
		t.Errorf("turn in progress = %+v", last)
	}
}

func TestPlayLogWaitForTakeout(t *testing.T) {
	s := Settings{StartScore: 501, OutMode: OutModeDouble}
	darts := []throw.Dart{hit(20, 3), hit(20, 3), hit(20, 3)}

	st, err := PlayLog(s, []int64{1, 2}, turn.Log{Entries: darts, WaitForTakeout: true})
	if err != nil {
		t.Fatal(err)
	}
	if !st.WaitingForTakeout || *st.CurrentPlayerID != 1 || st.DartsInTurn != 3 {
		t.Errorf("waiting=%v current=%d darts=%d, want player 1 waiting after 3 darts", st.WaitingForTakeout, *st.CurrentPlayerID, st.DartsInTurn)
	}
	// The turn is over while waiting: it counts in the stats and the history.
	if got := st.Players[0].Stats; got.Turns180 != 1 || got.HighestTurn != 180 {
		t.Errorf("stats while waiting = %+v, want the 180 counted", got)
	}
	if !st.Turns[0].Complete {
		t.Error("turn while waiting: not complete")
	}

	st, _ = PlayLog(s, []int64{1, 2}, turn.Log{Entries: append(darts, throw.Dart{Takeout: true}), WaitForTakeout: true})
	if st.WaitingForTakeout || *st.CurrentPlayerID != 2 {
		t.Errorf("after takeout: waiting=%v current=%d, want player 2", st.WaitingForTakeout, *st.CurrentPlayerID)
	}

	// A bust also waits: the darts are still in the board.
	st, _ = PlayLog(Settings{StartScore: 40, OutMode: OutModeDouble}, []int64{1, 2}, turn.Log{Entries: []throw.Dart{hit(20, 3)}, WaitForTakeout: true})
	if !st.WaitingForTakeout || st.Players[0].Remaining != 40 || st.DartsInTurn != 1 {
		t.Errorf("bust: waiting=%v remaining=%d darts=%d, want waiting with 40 left after 1 dart", st.WaitingForTakeout, st.Players[0].Remaining, st.DartsInTurn)
	}
}

func TestLegs(t *testing.T) {
	s := Settings{StartScore: 40, OutMode: OutModeDouble}
	log := turn.Log{Legs: 2, Entries: []throw.Dart{
		hit(20, 2), // leg 1: player 1 checks out and wins it at once
		miss,       // leg 2 is started by player 2
	}}
	st, err := PlayLog(s, []int64{1, 2}, log)
	if err != nil {
		t.Fatal(err)
	}
	if st.GameOver || st.Leg != 2 || *st.CurrentPlayerID != 2 || st.Players[0].Legs != 1 || st.Players[0].Remaining != 40 {
		t.Fatalf("after leg 1: over=%v leg=%d current=%d legs=%d remaining=%d, want leg 2 started by player 2 with the scores reset",
			st.GameOver, st.Leg, *st.CurrentPlayerID, st.Players[0].Legs, st.Players[0].Remaining)
	}

	// Player 1 wins the second leg, too, and with it the game.
	log.Entries = append(log.Entries, miss, miss, hit(20, 2))
	st, err = PlayLog(s, []int64{1, 2}, log)
	if err != nil {
		t.Fatal(err)
	}
	if !st.GameOver || *st.WinnerID != 1 || st.Players[0].Legs != 2 {
		t.Fatalf("over=%v legs=%d, want player 1 to win 2 legs to 0", st.GameOver, st.Players[0].Legs)
	}
	stats := st.Players[0].Stats
	if stats.BestLeg != 1 || stats.Points != 80 || stats.Checkout != 40 {
		t.Errorf("stats = %+v, want best leg 1 dart, 80 points over both legs", stats)
	}
	if _, err := PlayLog(s, []int64{1, 2}, turn.Log{Legs: 2, Entries: append(log.Entries, miss)}); !errors.Is(err, turn.ErrGameOver) {
		t.Errorf("dart after the game: got %v, want ErrGameOver", err)
	}

	// On a board the next leg waits for the darts to be pulled out.
	board := turn.Log{Legs: 2, WaitForTakeout: true, Entries: []throw.Dart{hit(20, 2)}}
	st, _ = PlayLog(s, []int64{1, 2}, board)
	if !st.WaitingForTakeout || !st.LegOver || st.Leg != 1 {
		t.Errorf("waiting=%v legOver=%v leg=%d, want leg 1 won and waiting for the takeout", st.WaitingForTakeout, st.LegOver, st.Leg)
	}
	board.Entries = append(board.Entries, throw.Dart{Takeout: true})
	st, _ = PlayLog(s, []int64{1, 2}, board)
	if st.WaitingForTakeout || st.Leg != 2 || *st.CurrentPlayerID != 2 {
		t.Errorf("after the takeout: waiting=%v leg=%d, want leg 2 started by player 2", st.WaitingForTakeout, st.Leg)
	}
}

func TestStatsPointsAfterBust(t *testing.T) {
	// The first dart of a busted turn doesn't count for the average.
	st, _ := Play(Settings{StartScore: 30, OutMode: OutModeDouble}, []int64{1}, []throw.Dart{hit(20, 1), hit(20, 1)})
	if got := st.Players[0].Stats.Points; got != 0 {
		t.Errorf("points = %d, want 0 after a bust", got)
	}
}
