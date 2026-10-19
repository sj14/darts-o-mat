package service

import (
	"net/url"
	"strconv"

	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/server/domain/tournament"
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
	Format     string      // FormatGame or a tournament.Format
	Mode       game.Mode   //
	StartScore uint64      // X01 only
	OutMode    x01.OutMode // X01 only
	Legs       int         // how many legs win
	Sets       int         // how many sets win, 1 is no sets
	BullOff    Choice
	TurnLimit  Choice
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
	case f.Legs != 0 && s.Legs != f.Legs:
		return false
	case f.Sets != 0 && s.Sets != f.Sets:
		return false
	case !f.BullOff.matches(s.BullOff):
		return false
	case !f.TurnLimit.matches(s.TurnSeconds > 0):
		return false
	}
	if f.StartScore != 0 || f.OutMode != 0 {
		// Only X01 has a start score and a way to check out.
		rules, ok := s.Rules.(*x01.Settings)
		if !ok || (f.StartScore != 0 && rules.StartScore != f.StartScore) || (f.OutMode != 0 && rules.OutMode != f.OutMode) {
			return false
		}
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

// LobbyFilterFromQuery reads the filter from the query of a request: format, mode,
// start_score, out_mode, legs, sets, bull_off and turn_limit (yes or no). A value that
// means nothing is left out rather than refused, so an old link still shows a list.
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
	switch mode := game.Mode(q.Get("mode")); mode {
	case game.ModeX01, game.ModeAroundTheClock:
		f.Mode = mode
	}
	if n, err := strconv.ParseUint(q.Get("start_score"), 10, 64); err == nil {
		f.StartScore = n
	}
	var out x01.OutMode
	if out.UnmarshalText([]byte(q.Get("out_mode"))) == nil {
		f.OutMode = out
	}
	if n, err := strconv.Atoi(q.Get("legs")); err == nil && n >= 1 && n <= game.MaxLegs {
		f.Legs = n
	}
	if n, err := strconv.Atoi(q.Get("sets")); err == nil && n >= 1 && n <= game.MaxSets {
		f.Sets = n
	}
	f.BullOff = choiceFrom(q.Get("bull_off"))
	f.TurnLimit = choiceFrom(q.Get("turn_limit"))
	return f
}

func choiceFrom(s string) Choice {
	switch c := Choice(s); c {
	case Yes, No:
		return c
	}
	return Any
}
