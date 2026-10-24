package stats

import (
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
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

// The form is the three-dart average in X01 and the hit rate in around the clock, over the
// results given, and needs MinFormGames of them.
func TestForm(t *testing.T) {
	t.Parallel()
	x01 := []Result{
		{Mode: game.ModeX01, Darts: 30, Points: 501},
		{Mode: game.ModeX01, Darts: 15, Points: 301},
	}
	if _, ok := Form(game.ModeX01, x01); ok {
		t.Error("a form from 2 games")
	}
	x01 = append(x01, Result{Mode: game.ModeX01, Darts: 15, Points: 198})
	if form, ok := Form(game.ModeX01, x01); !ok || form != 50 {
		t.Errorf("x01 form = %v, %v; want 50 (1000 points in 60 darts)", form, ok)
	}

	atc := []Result{
		{Mode: game.ModeAroundTheClock, Darts: 40, Hits: 20},
		{Mode: game.ModeAroundTheClock, Darts: 50, Hits: 20},
		{Mode: game.ModeAroundTheClock, Darts: 10, Hits: 0},
	}
	if form, ok := Form(game.ModeAroundTheClock, atc); !ok || form != 0.4 {
		t.Errorf("around the clock form = %v, %v; want 0.4 (40 hits in 100 darts)", form, ok)
	}
}
