package bot

import (
	"math"
	"math/rand/v2"
	"slices"
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

// The table of strengths is what a bot plays: the simulation of every spread gives about
// its values, and they fall as the spread grows. Print the measured table to update it.
func TestStrengths(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("plays some thousand legs")
	}
	modes := []game.Settings{
		{Mode: game.ModeX01, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}},
		{Mode: game.ModeAroundTheClock, Rules: &aroundtheclock.Settings{}},
		{Mode: game.ModeCricket, Rules: &cricket.Settings{Variant: cricket.VariantStandard}},
	}
	for _, settings := range modes {
		t.Run(string(settings.Mode), func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewPCG(1, 2))
			var measured []float64
			failed := false
			for i, spread := range strengthSpreads {
				got := measure(t, r, settings.WithDefaults(), spread)
				measured = append(measured, math.Round(got*100)/100)
				want := strengths[settings.Mode][i]
				if math.Abs(got-want) > 0.1*want+0.02 {
					failed = true
				}
				if i > 0 && strengths[settings.Mode][i] > strengths[settings.Mode][i-1] {
					t.Errorf("%d µm plays better than %d µm in the table", spread, strengthSpreads[i-1])
				}
			}
			if failed {
				t.Errorf("the table %v doesn't fit what the bots play: %#v", strengths[settings.Mode], measured)
			}
		})
	}
}

// TestStrength interpolates between the spreads measured.
func TestStrength(t *testing.T) {
	t.Parallel()
	for _, spreadUM := range []int{MinSpreadUM, 4_000, 6_500, 20_000, 79_000, MaxSpreadUM} {
		got := Strength(game.ModeX01, spreadUM)
		if i := slices.Index(strengthSpreads, spreadUM); i >= 0 && got != strengths[game.ModeX01][i] {
			t.Errorf("%d µm: %.2f, want the table's %.2f", spreadUM, got, strengths[game.ModeX01][i])
		}
		if got < Strength(game.ModeX01, MaxSpreadUM) || got > Strength(game.ModeX01, MinSpreadUM) {
			t.Errorf("%d µm: %.2f, outside the table", spreadUM, got)
		}
	}
	if a, b, c := Strength(game.ModeX01, 6_000), Strength(game.ModeX01, 6_500), Strength(game.ModeX01, 7_000); b < c || b > a {
		t.Errorf("6.5 mm plays %.2f, not between 6 mm (%.2f) and 7 mm (%.2f)", b, a, c)
	}
}

// The slider runs from the weakest to the strongest spread, steadily, in tenths of a mm, and
// Position is about the inverse of SpreadAt.
func TestSlider(t *testing.T) {
	t.Parallel()
	if got := SpreadAt(0); got != MaxSpreadUM {
		t.Errorf("left end %d µm, want %d", got, MaxSpreadUM)
	}
	if got := SpreadAt(SliderSteps); got != MinSpreadUM {
		t.Errorf("right end %d µm, want %d", got, MinSpreadUM)
	}
	if SpreadAt(-5) != MaxSpreadUM || SpreadAt(SliderSteps+5) != MinSpreadUM {
		t.Error("positions outside the slider aren't clamped to its ends")
	}
	if got := Position(DefaultSpreadUM); SpreadAt(got) != DefaultSpreadUM {
		t.Errorf("default spread at position %d is %d µm, want %d", got, SpreadAt(got), DefaultSpreadUM)
	}
	for pos := range SliderSteps + 1 {
		spreadUM := SpreadAt(pos)
		if spreadUM%sliderRoundingUM != 0 {
			t.Errorf("position %d is %d µm, not a tenth of a mm", pos, spreadUM)
		}
		if got := Position(spreadUM); math.Abs(float64(got-pos)) > 8 {
			// A tenth is coarse for the strong end (0.1 of 4 mm is 2.5 %, a step 0.3 %), so
			// the position is only about the same there.
			t.Errorf("position %d (%d µm) comes back as %d", pos, spreadUM, got)
		}
		if pos > 0 && spreadUM > SpreadAt(pos-1) {
			t.Errorf("position %d (%d µm) is weaker than %d (%d µm)", pos, spreadUM, pos-1, SpreadAt(pos-1))
		}
	}
	if err := CheckSpread(MinSpreadUM - 1); err == nil {
		t.Error("less than the smallest spread isn't refused")
	}
	if err := CheckSpread(MaxSpreadUM + 1); err == nil {
		t.Error("more than the largest spread isn't refused")
	}
}

// strengthLegs is how many legs TestStrengths lets a bot play with the spread. A strong bot
// finishes a leg in few darts, so it plays more of them to be as certain as a weak one,
// whose legs take up to 200 darts and, as every dart replays the leg, a lot of time.
func strengthLegs(spreadUM int) int {
	switch {
	case spreadUM <= 15_000:
		return 400
	case spreadUM <= 30_000:
		return 80
	}
	return 40
}

// measure lets a bot with the spread play legs alone and returns how it played, as the
// statistics count it over all its darts: the 3-dart average, the share of hits or the
// marks per round.
func measure(t *testing.T, r *rand.Rand, settings game.Settings, spreadUM int) float64 {
	t.Helper()
	g := game.Game{Settings: settings, Participants: []game.Participant{{PlayerID: 1, BotSpreadUM: spreadUM}}}
	var darts, score int
	for range strengthLegs(spreadUM) {
		switch st := playLeg(t, r, g).(type) {
		case x01.State:
			darts, score = darts+st.Players[0].Stats.Darts, score+int(st.Players[0].Stats.Points)
		case aroundtheclock.State:
			darts, score = darts+st.Players[0].Stats.Darts, score+st.Players[0].Stats.Hits
		case cricket.State:
			darts, score = darts+st.Players[0].Stats.Darts, score+st.Players[0].Stats.Marks
		}
	}
	switch settings.Mode {
	case game.ModeX01:
		return float64(score) * 3 / float64(darts)
	case game.ModeCricket:
		return cricket.MPR(score, darts)
	}
	return float64(score) / float64(darts)
}

// playLeg lets the bot throw a game of one leg alone and returns its state at the end, or
// after 200 darts, which a weak bot may not finish in.
func playLeg(t *testing.T, r *rand.Rand, g game.Game) game.State {
	t.Helper()
	var darts []throw.Dart
	for {
		st, err := g.Play(darts)
		if err != nil {
			t.Fatal(err)
		}
		aim, ok := g.Aim(st)
		if !ok || len(darts) == 200 {
			return st
		}
		d := Throw(r, aim, g.Participants[0].BotSpreadUM)
		if d.X == nil || d.Y == nil {
			t.Fatalf("dart %v without its position", d)
		}
		d.ID, d.PlayerID = int64(len(darts)+1), *st.Common().CurrentPlayerID
		darts = append(darts, d)
	}
}
