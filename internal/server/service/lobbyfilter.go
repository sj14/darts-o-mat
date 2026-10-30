package service

import (
	"net/url"
	"strconv"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// FormatGame is the LobbyFilter.Format of single games, as opposed to a tournament's format.
const FormatGame = "game"

// Choice is a yes-or-no filter that can also be left out.
type Choice string

const (
	Any Choice = ""
	Yes Choice = "yes"
	No  Choice = "no"
)

func (c Choice) matches(b bool) bool {
	return c == Any || (c == Yes) == b
}

// LobbyFilter narrows the list of open games and tournaments to those that fit what a
// player looks for. Whatever is left empty (the zero value) lets everything through.
type LobbyFilter struct {
	Format string    // FormatGame or a tournament.Format
	Mode   game.Mode //
	// Rules narrow down the settings of the mode, like game.Settings.Rules are those of a
	// game: the filter type that fits Mode (see rulesFilters), nil without a mode.
	Rules     RulesFilter
	Legs      int // how many legs win
	Sets      int // how many sets win, 1 is no sets
	BullOff   Choice
	BoardOnly Choice // everyone needs a board
	Fits      bool   // only those of about the own player's strength (OpenLobby.Fits)
}

// RulesFilter narrows down the settings of one mode (game.Rules). Its types are values, so
// that LobbyFilters can be compared.
type RulesFilter interface {
	// Match reports whether the rules pass the filter; rules of another mode don't.
	Match(game.Rules) bool
}

// X01Filter narrows down the settings of X01.
type X01Filter struct {
	StartScore uint64
	OutMode    x01.OutMode
}

func (f X01Filter) Match(r game.Rules) bool {
	rules, ok := r.(*x01.Settings)
	return ok && (f.StartScore == 0 || rules.StartScore == f.StartScore) && (f.OutMode == 0 || rules.OutMode == f.OutMode)
}

// AroundTheClockFilter narrows down the settings of around the clock.
type AroundTheClockFilter struct {
	Bull            Choice // the bull as the last target
	MultiplierSkips Choice
}

func (f AroundTheClockFilter) Match(r game.Rules) bool {
	rules, ok := r.(*aroundtheclock.Settings)
	return ok && f.Bull.matches(rules.Bull) && f.MultiplierSkips.matches(rules.MultiplierSkips)
}

// rulesFilters read the filter of a mode's settings from the query, one per mode.
var rulesFilters = map[game.Mode]func(url.Values) RulesFilter{
	game.ModeX01: func(q url.Values) RulesFilter {
		var f X01Filter
		if n, err := strconv.ParseUint(q.Get("start_score"), 10, 64); err == nil {
			f.StartScore = n
		}
		var out x01.OutMode
		if out.UnmarshalText([]byte(q.Get("out_mode"))) == nil {
			f.OutMode = out
		}
		return f
	},
	game.ModeAroundTheClock: func(q url.Values) RulesFilter {
		return AroundTheClockFilter{Bull: choiceFrom(q.Get("bull")), MultiplierSkips: choiceFrom(q.Get("multiplier_skips"))}
	},
}

// Active reports whether the filter leaves anything out.
func (f LobbyFilter) Active() bool {
	return f != LobbyFilter{}
}

// Match reports whether the lobby passes the filter.
func (f LobbyFilter) Match(l OpenLobby) bool {
	s := l.Settings
	switch {
	case f.Format == FormatGame && l.Tournament:
		return false
	case f.Format != "" && f.Format != FormatGame && (!l.Tournament || l.Format != tournament.Format(f.Format)):
		return false
	case f.Mode != "" && s.Mode != f.Mode:
		return false
	case f.Rules != nil && !f.Rules.Match(s.Rules):
		return false
	case f.Legs != 0 && s.Legs != f.Legs:
		return false
	case f.Sets != 0 && s.Sets != f.Sets:
		return false
	case !f.BullOff.matches(s.BullOff):
		return false
	case f.Fits && !l.Fits:
		return false
	case !f.BoardOnly.matches(l.BoardOnly):
		return false
	}
	return true
}

// Apply returns the lobbies that pass the filter, in their order.
func (f LobbyFilter) Apply(lobbies []OpenLobby) []OpenLobby {
	if !f.Active() {
		return lobbies
	}
	var kept []OpenLobby
	for _, l := range lobbies {
		if f.Match(l) {
			kept = append(kept, l)
		}
	}
	return kept
}

// LobbyFilterFromQuery reads the filter from the query of a request: format, mode and the
// settings of the mode (see rulesFilters: start_score and out_mode for x01, bull and
// multiplier_skips for around_the_clock, yes or no), legs, sets, bull_off and board_only
// (yes or no), and form=yes for about the own strength. The settings of a mode count only
// with the mode, as the form shows them only then. A value that means nothing is left out
// rather than refused, so an old link still shows a list.
func LobbyFilterFromQuery(q url.Values) LobbyFilter {
	var f LobbyFilter
	switch format := q.Get("format"); tournament.Format(format) {
	case tournament.Knockout, tournament.RoundRobin:
		f.Format = format
	default:
		if format == FormatGame {
			f.Format = format
		}
	}
	mode := game.Mode(q.Get("mode"))
	if rules, ok := rulesFilters[mode]; ok {
		f.Mode, f.Rules = mode, rules(q)
	}
	if n, err := strconv.Atoi(q.Get("legs")); err == nil && n >= 1 && n <= game.MaxLegs {
		f.Legs = n
	}
	if n, err := strconv.Atoi(q.Get("sets")); err == nil && n >= 1 && n <= game.MaxSets {
		f.Sets = n
	}
	f.BullOff = choiceFrom(q.Get("bull_off"))
	f.BoardOnly = choiceFrom(q.Get("board_only"))
	f.Fits = q.Get("form") == string(Yes)
	return f
}

func choiceFrom(s string) Choice {
	switch c := Choice(s); c {
	case Yes, No:
		return c
	}
	return Any
}
