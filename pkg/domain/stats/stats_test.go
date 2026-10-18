package stats

import (
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
)

func TestRecordAveragesDurations(t *testing.T) {
	start := time.Date(2026, 1, 1, 20, 0, 0, 0, time.UTC)
	span := func(from, to time.Duration) turn.Span {
		return turn.Span{Start: start.Add(from), End: start.Add(to)}
	}
	var r Record
	r.add(turn.Record{}, true, turn.Times{
		Game: span(0, 10*time.Minute),
		Legs: []turn.LegTime{{Leg: 1, Span: span(0, 4*time.Minute)}, {Leg: 2, Span: span(4*time.Minute, 10*time.Minute)}},
	})
	r.add(turn.Record{}, false, turn.Times{Game: span(0, 20*time.Minute), Legs: []turn.LegTime{{Leg: 1, Span: span(0, 20*time.Minute)}}})
	// A game without times doesn't pull the averages down.
	r.add(turn.Record{}, false, turn.Times{})

	if r.Games != 3 || r.AvgGame != 15*time.Minute || r.AvgLeg != 10*time.Minute {
		t.Errorf("games = %d, avg game = %v, avg leg = %v; want 3, 15m, 10m", r.Games, r.AvgGame, r.AvgLeg)
	}
}
