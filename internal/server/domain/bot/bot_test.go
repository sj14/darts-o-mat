package bot

import (
	"math/rand/v2"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/cricket"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// The middle of every field lies in the field.
func TestMiddle(t *testing.T) {
	t.Parallel()
	fields := []throw.Dart{{Number: dartboard.BullNumber, Segment: dartboard.Bull}, {Number: dartboard.BullNumber, Segment: dartboard.Bullseye}}
	for _, n := range dartboard.Segments {
		for _, s := range []dartboard.Segment{dartboard.Inner, dartboard.Triple, dartboard.Outer, dartboard.Double} {
			fields = append(fields, throw.Dart{Number: n, Segment: s})
		}
	}
	for _, f := range fields {
		want := f
		if want.Segment == dartboard.Bull {
			want.Segment = dartboard.Bullseye // the bull's middle is the bullseye
		}
		if got := dartboard.Score(middle(f)); got.Number != want.Number || got.Segment != want.Segment {
			t.Errorf("middle of %v scores %v", f, got)
		}
	}
	single := throw.Dart{Number: 20, Segment: dartboard.Single}
	if got := dartboard.Score(middle(single)); got.Number != 20 || got.Segment != dartboard.Outer {
		t.Errorf("middle of a single 20 scores %v, want the outer single", got)
	}
}

// A bot plays the better the smaller its spread, in every mode, and the darts keep where
// they landed.
func TestSpreadPlaysBetter(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(1, 2))
	modes := []struct {
		name     string
		settings game.Settings
		measure  func(st game.State) float64 // the higher the better
	}{
		{"x01", game.Settings{Mode: game.ModeX01, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}},
			func(st game.State) float64 { return st.(x01.State).Players[0].Stats.Average }},
		{"around the clock", game.Settings{Mode: game.ModeAroundTheClock, Rules: &aroundtheclock.Settings{}},
			func(st game.State) float64 { return st.(aroundtheclock.State).Players[0].Stats.HitRate }},
		{"cricket", game.Settings{Mode: game.ModeCricket, Rules: &cricket.Settings{Variant: cricket.VariantStandard}},
			func(st game.State) float64 { return st.(cricket.State).Players[0].Stats.MPR }},
	}
	for _, m := range modes {
		previous := 0.0
		for _, spread := range []int{40, 25, 15, 8} {
			g := game.Game{Settings: m.settings.WithDefaults(), Participants: []game.Participant{{PlayerID: 1, BotSpread: spread}}}
			total := 0.0
			const legs = 20
			for range legs {
				st := playLeg(t, r, g)
				total += m.measure(st)
			}
			got := total / legs
			t.Logf("%s, spread %d mm: %.2f", m.name, spread, got)
			if got <= previous {
				t.Errorf("%s: spread %d mm plays %.2f, not better than the larger spread before (%.2f)", m.name, spread, got, previous)
			}
			previous = got
		}
	}
}

// playLeg lets the bot throw a game of one leg alone and returns its state at the end.
func playLeg(t *testing.T, r *rand.Rand, g game.Game) game.State {
	t.Helper()
	var darts []throw.Dart
	for range 1000 {
		st, err := g.Play(darts)
		if err != nil {
			t.Fatal(err)
		}
		aim, ok := g.Aim(st)
		if !ok {
			return st
		}
		d := Throw(r, aim, g.Participants[0].BotSpread)
		if d.X == nil || d.Y == nil {
			t.Fatalf("dart %v without its position", d)
		}
		d.ID, d.PlayerID = int64(len(darts)+1), *st.Common().CurrentPlayerID
		darts = append(darts, d)
	}
	t.Fatal("the bot didn't finish in 1000 darts")
	return nil
}
