package service

import (
	"net/url"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

func lobbyOf(s game.Settings, t bool, f tournament.Format) OpenLobby {
	return OpenLobby{Settings: s, Tournament: t, Format: f}
}

func TestLobbyFilterMatch(t *testing.T) {
	t.Parallel()
	x501 := game.Settings{Mode: game.ModeX01, Legs: 3, Sets: 1, BullOff: true, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}}
	x301 := game.Settings{Mode: game.ModeX01, Legs: 1, Sets: 2, TurnSeconds: 60, Rules: &x01.Settings{StartScore: 301, OutMode: x01.OutModeSingle}}
	atc := game.Settings{Mode: game.ModeAroundTheClock, Legs: 1, Sets: 1, Rules: &aroundtheclock.Settings{}}

	tests := []struct {
		name   string
		filter LobbyFilter
		lobby  OpenLobby
		want   bool
	}{
		{"board only, wanted", LobbyFilter{BoardOnly: Yes}, OpenLobby{Settings: x501, BoardOnly: true}, true},
		{"board only, not wanted", LobbyFilter{BoardOnly: No}, OpenLobby{Settings: x501, BoardOnly: true}, false},
		{"empty filter", LobbyFilter{}, lobbyOf(x501, false, ""), true},
		{"single games only, a game", LobbyFilter{Format: FormatGame}, lobbyOf(x501, false, ""), true},
		{"single games only, a tournament", LobbyFilter{Format: FormatGame}, lobbyOf(x501, true, tournament.Knockout), false},
		{"knockout, a knockout", LobbyFilter{Format: "knockout"}, lobbyOf(x501, true, tournament.Knockout), true},
		{"knockout, a round robin", LobbyFilter{Format: "knockout"}, lobbyOf(x501, true, tournament.RoundRobin), false},
		{"knockout, a game", LobbyFilter{Format: "knockout"}, lobbyOf(x501, false, ""), false},
		{"mode", LobbyFilter{Mode: game.ModeAroundTheClock}, lobbyOf(x501, false, ""), false},
		{"start score", LobbyFilter{StartScore: 501}, lobbyOf(x501, false, ""), true},
		{"other start score", LobbyFilter{StartScore: 301}, lobbyOf(x501, false, ""), false},
		{"start score never fits Around the Clock", LobbyFilter{StartScore: 501}, lobbyOf(atc, false, ""), false},
		{"check-out", LobbyFilter{OutMode: x01.OutModeDouble}, lobbyOf(x501, false, ""), true},
		{"other check-out", LobbyFilter{OutMode: x01.OutModeMaster}, lobbyOf(x501, false, ""), false},
		{"legs", LobbyFilter{Legs: 3}, lobbyOf(x501, false, ""), true},
		{"other legs", LobbyFilter{Legs: 2}, lobbyOf(x501, false, ""), false},
		{"no sets", LobbyFilter{Sets: 1}, lobbyOf(x301, false, ""), false},
		{"sets", LobbyFilter{Sets: 2}, lobbyOf(x301, false, ""), true},
		{"with bull-off", LobbyFilter{BullOff: Yes}, lobbyOf(x501, false, ""), true},
		{"without bull-off", LobbyFilter{BullOff: No}, lobbyOf(x501, false, ""), false},
		{"all of it", LobbyFilter{Format: FormatGame, Mode: game.ModeX01, StartScore: 501, OutMode: x01.OutModeDouble, Legs: 3, Sets: 1, BullOff: Yes}, lobbyOf(x501, false, ""), true},
	}
	for _, tt := range tests {
		if got := tt.filter.Match(tt.lobby); got != tt.want {
			t.Errorf("%s: Match = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestLobbyFilterApply(t *testing.T) {
	t.Parallel()
	a := OpenLobby{ID: 1, Settings: game.Settings{Mode: game.ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}}}
	b := OpenLobby{ID: 2, Settings: game.Settings{Mode: game.ModeAroundTheClock, Legs: 1, Sets: 1, Rules: &aroundtheclock.Settings{}}}
	c := OpenLobby{ID: 3, Settings: a.Settings}
	all := []OpenLobby{a, b, c}

	if got := (LobbyFilter{}).Apply(all); len(got) != 3 {
		t.Errorf("no filter kept %d of 3", len(got))
	}
	got := LobbyFilter{Mode: game.ModeX01}.Apply(all)
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Errorf("X01 kept %+v, want 1 and 3 in their order", got)
	}
}

func TestLobbyFilterFromQuery(t *testing.T) {
	t.Parallel()
	q, _ := url.ParseQuery("format=round_robin&mode=x01&start_score=501&out_mode=double&legs=3&sets=2&bull_off=yes&form=yes")
	want := LobbyFilter{Format: "round_robin", Mode: game.ModeX01, StartScore: 501, OutMode: x01.OutModeDouble, Legs: 3, Sets: 2, BullOff: Yes, Fits: true}
	if got := LobbyFilterFromQuery(q); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	// What means nothing is left out, so an old link still shows a list.
	q, _ = url.ParseQuery("format=chess&mode=poker&start_score=abc&out_mode=triple&legs=0&sets=999&bull_off=maybe&form=maybe")
	if got := LobbyFilterFromQuery(q); got.Active() {
		t.Errorf("nonsense made a filter: %+v", got)
	}
	if got := LobbyFilterFromQuery(url.Values{"format": {"game"}}); got.Format != FormatGame || !got.Active() {
		t.Errorf("single games: %+v", got)
	}
	if LobbyFilterFromQuery(nil).Active() {
		t.Error("no query made a filter")
	}
}
