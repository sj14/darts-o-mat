package x01

import (
	"errors"
	"slices"
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
		wantWinners   []int64
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
			name:          "after a checkout the remaining players may still throw",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 1), hit(10, 2)},
			wantRemaining: []uint64{0, 40},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "game is over at the end of the round",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 1), hit(10, 2), miss, miss, miss},
			wantRemaining: []uint64{0, 40},
			wantRound:     1,
			wantWinners:   []int64{1},
		},
		{
			name:          "draw when both check out in the same round",
			settings:      Settings{StartScore: 40, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(20, 2), hit(10, 1), hit(15, 2)},
			wantRemaining: []uint64{0, 0},
			wantRound:     1,
			wantWinners:   []int64{1, 2},
		},
		{
			name:          "bullseye counts as double",
			settings:      Settings{StartScore: 50, OutMode: OutModeDouble},
			darts:         []throw.Dart{hit(throw.Bull, 2)},
			wantRemaining: []uint64{0, 50},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "checkout by the last player ends the game",
			settings:      Settings{StartScore: 20, OutMode: OutModeSingle},
			darts:         []throw.Dart{miss, miss, miss, hit(19, 1), hit(1, 1)},
			wantRemaining: []uint64{20, 0},
			wantRound:     1,
			wantWinners:   []int64{2},
		},
		{
			name:          "master out finishes on a double",
			settings:      Settings{StartScore: 6, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(3, 2)},
			wantRemaining: []uint64{0, 6},
			wantCurrent:   2,
			wantRound:     1,
		},
		{
			name:          "master out finishes on a triple",
			settings:      Settings{StartScore: 12, OutMode: OutModeMaster},
			darts:         []throw.Dart{hit(3, 2), hit(2, 3)},
			wantRemaining: []uint64{0, 12},
			wantCurrent:   2,
			wantRound:     1,
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
			if !slices.Equal(st.WinnerIDs, tt.wantWinners) && len(st.WinnerIDs)+len(tt.wantWinners) > 0 {
				t.Errorf("winners = %v, want %v", st.WinnerIDs, tt.wantWinners)
			}
			if st.GameOver != (len(tt.wantWinners) > 0) {
				t.Errorf("game over = %v, want %v", st.GameOver, len(tt.wantWinners) > 0)
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
