package aroundtheclock

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

// hitsUpTo returns single hits on 1 to n.
func hitsUpTo(n uint8) []throw.Dart {
	var darts []throw.Dart
	for i := uint8(1); i <= n; i++ {
		darts = append(darts, hit(i, 1))
	}
	return darts
}

func TestPlay(t *testing.T) {
	tests := []struct {
		name        string
		settings    Settings
		darts       []throw.Dart
		wantTargets []uint8
		wantCurrent int64 // 0 means the game is over
		wantWinners []int64
	}{
		{
			name:        "no darts",
			wantTargets: []uint8{1, 1},
			wantCurrent: 1,
		},
		{
			name:        "wrong number does not advance",
			darts:       []throw.Dart{hit(2, 1), hit(1, 1), hit(2, 1)},
			wantTargets: []uint8{3, 1},
			wantCurrent: 2,
		},
		{
			name:        "double advances one without skips",
			darts:       []throw.Dart{hit(1, 2)},
			wantTargets: []uint8{2, 1},
			wantCurrent: 1,
		},
		{
			name:        "multiplier skips",
			settings:    Settings{MultiplierSkips: true},
			darts:       []throw.Dart{hit(1, 2), hit(3, 3)},
			wantTargets: []uint8{6, 1},
			wantCurrent: 1,
		},
		{
			name:        "finished after 20 without bull, player 2 may still throw",
			darts:       oneplayer(hitsUpTo(20)),
			wantTargets: []uint8{0, 1},
			wantCurrent: 2,
		},
		{
			name:        "game is over at the end of the round",
			darts:       append(oneplayer(hitsUpTo(20)), hit(1, 1), hit(2, 1), hit(3, 1)),
			wantTargets: []uint8{0, 4},
			wantWinners: []int64{1},
		},
		{
			name:        "bull is the last target",
			settings:    Settings{Bull: true},
			darts:       oneplayer(hitsUpTo(20)),
			wantTargets: []uint8{throw.Bull, 1},
			wantCurrent: 1,
		},
		{
			name:        "win with bull",
			settings:    Settings{Bull: true},
			darts:       oneplayer(append(hitsUpTo(20), hit(throw.Bull, 1))),
			wantTargets: []uint8{0, 1},
			wantCurrent: 2,
		},
		{
			name:        "skipping past 20 goes to bull",
			settings:    Settings{Bull: true, MultiplierSkips: true},
			darts:       oneplayer(append(hitsUpTo(18), hit(19, 3))),
			wantTargets: []uint8{throw.Bull, 1},
			wantCurrent: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st, err := Play(tt.settings, []int64{1, 2}, tt.darts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for i, want := range tt.wantTargets {
				if got := st.Players[i].Target; got != want {
					t.Errorf("player %d target = %d, want %d", i+1, got, want)
				}
			}
			var current int64
			if st.CurrentPlayerID != nil {
				current = *st.CurrentPlayerID
			}
			if current != tt.wantCurrent {
				t.Errorf("current player = %d, want %d", current, tt.wantCurrent)
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

// oneplayer interleaves the darts of player 1 with three misses of player 2 after each turn.
func oneplayer(darts []throw.Dart) []throw.Dart {
	var result []throw.Dart
	for i, d := range darts {
		result = append(result, d)
		if i%3 == 2 && i != len(darts)-1 {
			result = append(result, throw.Dart{Miss: true}, throw.Dart{Miss: true}, throw.Dart{Miss: true})
		}
	}
	return result
}

func TestPlayAfterWin(t *testing.T) {
	_, err := Play(Settings{}, []int64{1}, append(hitsUpTo(20), throw.Dart{Miss: true}))
	if !errors.Is(err, turn.ErrGameOver) {
		t.Errorf("got %v, want ErrGameOver", err)
	}
}

func TestStats(t *testing.T) {
	darts := []throw.Dart{hit(1, 1), hit(5, 1), hit(2, 1), throw.Dart{Miss: true}}
	st, err := Play(Settings{}, []int64{1, 2}, darts)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Players[0].Stats; got.Darts != 3 || got.Hits != 2 || got.HitRate != 2.0/3 {
		t.Errorf("player 1 stats = %+v", got)
	}
	if got := st.Players[1].Stats; got.Darts != 1 || got.Hits != 0 || got.HitRate != 0 {
		t.Errorf("player 2 stats = %+v", got)
	}
}
