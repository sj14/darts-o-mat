package game

import (
	"fmt"
	"reflect"

	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/cricket"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

// Rules are the settings of a game mode (x01.Settings, aroundtheclock.Settings,
// cricket.Settings, ...), the part of Settings that differs from mode to mode.
// Settings.Rules holds a pointer to the mode's own type.
type Rules interface {
	Validate() error
}

// A mode is what the game package needs to know of a game mode. To add one: its package
// with Settings, PlayLog and Aim, its constant above and an entry here, then its table in
// the store (store/settings.go) and its filter of the open games (service.rulesFilters).
type mode struct {
	// rules returns empty rules of the mode, to be filled in.
	rules func() Rules
	// play replays the log of a game of this mode, r being what rules returned.
	play func(r Rules, playerIDs []int64, log turn.Log) (State, error)
	// aim returns the field a bot aims at with its next dart, in the state play returned.
	aim func(r Rules, st State, playerID int64) throw.Dart
}

var modes = map[Mode]mode{
	ModeX01: {
		rules: func() Rules { return &x01.Settings{} },
		play: func(r Rules, playerIDs []int64, log turn.Log) (State, error) {
			return x01.PlayLog(*r.(*x01.Settings), playerIDs, log)
		},
		aim: func(r Rules, st State, playerID int64) throw.Dart {
			return x01.Aim(*r.(*x01.Settings), st.(x01.State), playerID)
		},
	},
	ModeAroundTheClock: {
		rules: func() Rules { return &aroundtheclock.Settings{} },
		play: func(r Rules, playerIDs []int64, log turn.Log) (State, error) {
			return aroundtheclock.PlayLog(*r.(*aroundtheclock.Settings), playerIDs, log)
		},
		aim: func(r Rules, st State, playerID int64) throw.Dart {
			return aroundtheclock.Aim(*r.(*aroundtheclock.Settings), st.(aroundtheclock.State), playerID)
		},
	},
	ModeCricket: {
		rules: func() Rules { return &cricket.Settings{} },
		play: func(r Rules, playerIDs []int64, log turn.Log) (State, error) {
			return cricket.PlayLog(*r.(*cricket.Settings), playerIDs, log)
		},
		aim: func(r Rules, st State, playerID int64) throw.Dart {
			return cricket.Aim(*r.(*cricket.Settings), st.(cricket.State), playerID)
		},
	},
}

// NewRules returns empty rules of the mode, nil for a mode that doesn't exist. Decoding
// JSON into Settings needs them first, see Settings.Rules.
func NewRules(m Mode) Rules {
	if def, ok := modes[m]; ok {
		return def.rules()
	}
	return nil
}

// validateRules checks that the rules are those of the mode, and valid.
func (s Settings) validateRules() error {
	def, ok := modes[s.Mode]
	if !ok {
		return fmt.Errorf("unknown mode %q", s.Mode)
	}
	if reflect.TypeOf(s.Rules) != reflect.TypeOf(def.rules()) {
		return fmt.Errorf("mode %s needs its own rules", s.Mode)
	}
	return s.Rules.Validate()
}
