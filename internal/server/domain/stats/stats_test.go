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

// The form is the three-dart average in X01, the hit rate in around the clock and the marks
// per round in cricket, over the results given, and needs MinFormGames of them.
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

	cr := []Result{
		{Mode: game.ModeCricket, Darts: 30, Marks: 30},
		{Mode: game.ModeCricket, Darts: 15, Marks: 15},
		{Mode: game.ModeCricket, Darts: 15, Marks: 0},
	}
	if form, ok := Form(game.ModeCricket, cr); !ok || form != 2.25 {
		t.Errorf("cricket form = %v, %v; want 2.25 (45 marks in 60 darts)", form, ok)
	}
	if _, ok := Form(game.ModeCricket, cr[:2]); ok {
		t.Error("a cricket form from 2 games")
	}
}

func TestSummaryAddsCricket(t *testing.T) {
	t.Parallel()
	var s Summary
	s.AddResult(Result{Mode: game.ModeCricket, Won: true, BestLeg: 12, Darts: 12, Marks: 12})
	s.AddResult(Result{Mode: game.ModeCricket, BestLeg: 9, Darts: 0, Marks: 0})
	s.AddResult(Result{Mode: game.ModeX01, Darts: 3, Points: 60})
	s.AddTournamentResult(TournamentResult{Mode: game.ModeCricket, Won: true})
	s.AddQuit(Quit{Mode: game.ModeCricket})

	c := s.Cricket
	if c.Games != 2 || c.Wins != 1 || c.BestLeg != 9 || c.Darts != 12 || c.Marks != 12 || c.MPR != 3 {
		t.Errorf("cricket = %+v; want 2 games, 1 win, best leg 9, 12 darts, 12 marks, MPR 3", c)
	}
	if c.Tournaments != 1 || c.TournamentWins != 1 || c.GamesLeft != 1 {
		t.Errorf("cricket tournaments = %d/%d, left = %d; want 1/1, 1", c.Tournaments, c.TournamentWins, c.GamesLeft)
	}
	if s.X01.Games != 1 || s.X01.Tournaments != 0 || s.X01.GamesLeft != 0 || s.AroundTheClock.Any() {
		t.Errorf("another mode counted cricket's: x01 %+v, around the clock %+v", s.X01, s.AroundTheClock)
	}
	if (Summary{}).Cricket.MPR != 0 {
		t.Error("MPR without darts isn't 0")
	}
}

func TestFormStep(t *testing.T) {
	t.Parallel()
	for mode, want := range map[game.Mode]float64{
		game.ModeX01:            5,
		game.ModeAroundTheClock: 0.05,
		game.ModeCricket:        0.3,
	} {
		if got := FormStep(mode); got != want {
			t.Errorf("FormStep(%s) = %v, want %v", mode, got, want)
		}
	}
}

// The forms may differ by one step at first, by one more every FormWiden, and by anything
// after FormAnyone: they fit once one side waited FormWait.
func TestFormWait(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		mode   game.Mode
		a, b   float64
		waited time.Duration
		want   bool
	}{
		{game.ModeX01, 50, 55, 0, true},
		{game.ModeX01, 50, 56, 0, false},
		{game.ModeX01, 50, 60, 29 * time.Second, false},
		{game.ModeX01, 50, 60, 30 * time.Second, true},
		{game.ModeX01, 40, 70, 150 * time.Second, true}, // 6 steps
		{game.ModeX01, 20, 90, 179 * time.Second, false},
		{game.ModeX01, 20, 90, FormAnyone, true},
		{game.ModeAroundTheClock, 0.40, 0.45, 0, true},
		{game.ModeAroundTheClock, 0.40, 0.46, 0, false},
		{game.ModeAroundTheClock, 0.30, 0.40, time.Minute, true},
		{game.ModeCricket, 2.0, 2.3, 0, true},
		{game.ModeCricket, 2.0, 2.4, 0, false},
		{game.ModeCricket, 2.0, 2.4, 30 * time.Second, true},
	} {
		if got := FormWait(c.mode, c.a, c.b) <= c.waited; got != c.want {
			t.Errorf("%s: %v and %v after %v fit: %v, want %v", c.mode, c.a, c.b, c.waited, got, c.want)
		}
	}
}

func TestFormWaitValues(t *testing.T) {
	t.Parallel()
	if got := FormWait(game.ModeX01, 70, 40); got != 150*time.Second {
		t.Errorf("40 and 70: %v, want 2m30s (6 steps)", got)
	}
	if got := FormWait(game.ModeX01, 50, 52); got != 0 {
		t.Errorf("50 and 52: %v, want 0", got)
	}
	if got := FormWait(game.ModeX01, 20, 90); got != FormAnyone {
		t.Errorf("20 and 90: %v, want FormAnyone", got)
	}
}

// The statistics show the games against people or those against bots, each on its own.
func TestOpponents(t *testing.T) {
	t.Parallel()
	results := []Result{
		{Mode: game.ModeX01, Won: true, Legs: 1, Darts: 30, Points: 501},
		{Mode: game.ModeX01, Won: true, Legs: 1, Darts: 15, Points: 501, Bots: true},
		{Mode: game.ModeX01, Darts: 30, Points: 400, Bots: true},
	}
	for _, tc := range []struct {
		opponents   Opponents
		games, wins int
	}{
		{AgainstPeople, 1, 1},
		{AgainstBots, 2, 1},
	} {
		var s Summary
		for _, r := range results {
			if tc.opponents.Counts(r) {
				s.AddResult(r)
			}
		}
		if s.X01.Games != tc.games || s.X01.Wins != tc.wins {
			t.Errorf("against %q: games %d, wins %d; want %d, %d", tc.opponents, s.X01.Games, s.X01.Wins, tc.games, tc.wins)
		}
	}
}
