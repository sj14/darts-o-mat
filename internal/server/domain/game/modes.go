package game

import (
	"fmt"
	"reflect"

	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// Rules are the settings of a game mode (x01.Settings, aroundtheclock.Settings, ...), the
// part of Settings that differs from mode to mode. Settings.Rules holds a pointer to the
// mode's own type.
type Rules interface {
	Validate() error
}

// A mode is what the game package needs to know of a game mode. To add one: its package
// with Settings and PlayLog, its constant above and an entry here, then its table in the
// store (store/settings.go).
type mode struct {
	// rules returns empty rules of the mode, to be filled in.
	rules func() Rules
	// play replays the log of a game of this mode, r being what rules returned.
	play func(r Rules, playerIDs []int64, log turn.Log) (State, error)
}

var modes = map[Mode]mode{
	ModeX01: {
		rules: func() Rules { return &x01.Settings{} },
		play: func(r Rules, playerIDs []int64, log turn.Log) (State, error) {
			return x01.PlayLog(*r.(*x01.Settings), playerIDs, log)
		},
	},
	ModeAroundTheClock: {
		rules: func() Rules { return &aroundtheclock.Settings{} },
		play: func(r Rules, playerIDs []int64, log turn.Log) (State, error) {
			return aroundtheclock.PlayLog(*r.(*aroundtheclock.Settings), playerIDs, log)
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
