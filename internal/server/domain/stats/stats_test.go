package stats

import (
	"testing"
	"time"
)

func TestRecordAveragesDurations(t *testing.T) {
	t.Parallel()
	var r Record
	r.add(Result{Won: true, GameTime: 10 * time.Minute, LegTime: 10 * time.Minute, TimedLegs: 2})
	r.add(Result{GameTime: 20 * time.Minute, LegTime: 20 * time.Minute, TimedLegs: 1})
	// A game without times doesn't pull the averages down.
	r.add(Result{})

	if r.Games != 3 || r.Wins != 1 || r.AvgGame != 15*time.Minute || r.AvgLeg != 10*time.Minute {
		t.Errorf("games = %d, wins = %d, avg game = %v, avg leg = %v; want 3, 1, 15m, 10m", r.Games, r.Wins, r.AvgGame, r.AvgLeg)
	}
}
