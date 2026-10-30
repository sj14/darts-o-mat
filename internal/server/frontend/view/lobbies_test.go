package view

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// The filter shows the settings of the mode chosen only, and none without a mode.
func TestLobbyFilterShowsTheSettingsOfTheMode(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		filter    service.LobbyFilter
		want, not []string
	}{
		{service.LobbyFilter{}, nil, []string{`name="start_score"`, `name="out_mode"`, `name="bull"`, `name="multiplier_skips"`}},
		{service.LobbyFilter{Mode: game.ModeX01, Rules: service.X01Filter{}}, []string{`name="start_score"`, `name="out_mode"`}, []string{`name="bull"`, `name="multiplier_skips"`}},
		{service.LobbyFilter{Mode: game.ModeAroundTheClock, Rules: service.AroundTheClockFilter{}}, []string{`name="bull"`, `name="multiplier_skips"`}, []string{`name="start_score"`, `name="out_mode"`}},
	} {
		var b bytes.Buffer
		if err := lobbyFilter(tt.filter).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		got := b.String()
		for _, s := range tt.want {
			if !strings.Contains(got, s) {
				t.Errorf("mode %q: no %s", tt.filter.Mode, s)
			}
		}
		for _, s := range tt.not {
			if strings.Contains(got, s) {
				t.Errorf("mode %q: shows %s", tt.filter.Mode, s)
			}
		}
	}
}
