package service

import (
	"context"
	"reflect"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// The start form shows the settings of the account's last game or tournament of the mode,
// the defaults before the first; a quick match doesn't count.
func TestNewSettings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	guest, _ := s.CreateGuest(ctx, anna, "Annas Gast")

	settingsOf := func(acc account.Account, mode game.Mode) game.Settings {
		t.Helper()
		got, err := s.NewSettings(ctx, acc, mode)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for _, mode := range []game.Mode{game.ModeX01, game.ModeAroundTheClock} {
		if got := settingsOf(anna, mode); !reflect.DeepEqual(got, DefaultSettings(mode)) {
			t.Errorf("%s before the first game = %+v, want the defaults", mode, got)
		}
	}

	played := game.Settings{Mode: game.ModeX01, Legs: 3, Sets: 2, BullOff: true, TurnSeconds: 60,
		Rules: &x01.Settings{StartScore: 701, OutMode: x01.OutModeMaster}}
	if _, err := s.CreateGame(ctx, anna, game.Game{Settings: played}, 0, nil); err != nil {
		t.Fatal(err)
	}
	if got := settingsOf(anna, game.ModeX01); !reflect.DeepEqual(got, played) {
		t.Errorf("X01 after a game = %+v, want %+v", got, played)
	}
	if got := settingsOf(anna, game.ModeAroundTheClock); !reflect.DeepEqual(got, DefaultSettings(game.ModeAroundTheClock)) {
		t.Errorf("around the clock after an X01 game = %+v, want the defaults", got)
	}
	if got := settingsOf(ben, game.ModeX01); !reflect.DeepEqual(got, DefaultSettings(game.ModeX01)) {
		t.Errorf("another account's X01 = %+v, want the defaults", got)
	}

	cup := game.Settings{Mode: game.ModeAroundTheClock, Legs: 2, Sets: 1,
		Rules: &aroundtheclock.Settings{MultiplierSkips: true}}
	tt := tournament.Tournament{Format: tournament.Knockout, Settings: cup}
	if _, err := s.CreateTournament(ctx, anna, tt, guest.ID, nil); err != nil {
		t.Fatal(err)
	}
	if got := settingsOf(anna, game.ModeAroundTheClock); !reflect.DeepEqual(got, cup) {
		t.Errorf("around the clock after a tournament = %+v, want %+v", got, cup)
	}

	before := settingsOf(ben, game.ModeX01)
	if _, err := s.QuickMatch(ctx, ben, game.ModeX01, nil); err != nil {
		t.Fatal(err)
	}
	if got := settingsOf(ben, game.ModeX01); !reflect.DeepEqual(got, before) {
		t.Errorf("X01 after a quick match = %+v, want %+v as before", got, before)
	}
}
