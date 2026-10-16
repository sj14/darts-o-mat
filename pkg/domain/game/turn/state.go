package turn

import "time"

// State is the part of a game's state that is the same for all modes: whose turn it
// is, legs, bulling off and the end of the game. The modes embed it in their state.
type State struct {
	CurrentPlayerID *int64 `json:"current_player_id"` // nil once the game is over
	Round           int    `json:"round"`
	DartsInTurn     int    `json:"darts_in_turn"`
	Set             int    `json:"set"` // starts at 1; always 1 without sets
	Leg             int    `json:"leg"` // in the current set, starts at 1
	GameOver        bool   `json:"game_over"`
	WinnerID        *int64 `json:"winner_id"` // nil until the game is over
	// WaitingForTakeout is set when the turn is over, but the darts weren't pulled out yet
	// (only in games played on a board, see Log.WaitForTakeout).
	WaitingForTakeout bool `json:"waiting_for_takeout"`
	// LegOver is set while waiting for the takeout after a leg was won, SetOver when
	// that leg also won the set.
	LegOver bool `json:"leg_over"`
	SetOver bool `json:"set_over"`
	// LegWinner is who won a leg with the last dart, while the game goes on; SetWinner
	// is also set when that leg won a set.
	LegWinner *int64 `json:"leg_winner,omitempty"`
	SetWinner *int64 `json:"set_winner,omitempty"`
	// BullingOff is set while the players bull off for who starts; CurrentPlayerID throws next.
	BullingOff bool `json:"bulling_off"`
	// BullOff lists the darts thrown to bull off.
	BullOff []BullOffDart `json:"bull_off"`
	// BullOffTie lists the players who are equally close and throw again, unless the
	// players decide who was closer.
	BullOffTie []int64 `json:"bull_off_tie,omitempty"`
	// Throwers is the player who threw each entry of the log, 0 for takeouts.
	Throwers []int64 `json:"-"`
	// Time is when the game was played so far, Legs and Sets when each finished leg and
	// set was played, in order.
	Time Span      `json:"time"`
	Legs []LegTime `json:"legs"`
	Sets []Span    `json:"sets"`
}

// Span is when a part of a game was played: from its first dart to its last.
type Span struct {
	Start time.Time `json:"start,omitzero"`
	End   time.Time `json:"end,omitzero"`
}

// Duration returns how long it lasted, 0 if unknown (e.g. darts without a time).
func (s Span) Duration() time.Duration {
	if s.Start.IsZero() || s.End.IsZero() {
		return 0
	}
	return s.End.Sub(s.Start)
}

// Join returns the span from the earlier start to the later end, e.g. of a tournament's
// games; unknown times (zero) don't count.
func (s Span) Join(o Span) Span {
	if s.Start.IsZero() || (!o.Start.IsZero() && o.Start.Before(s.Start)) {
		s.Start = o.Start
	}
	if o.End.After(s.End) {
		s.End = o.End
	}
	return s
}

// LegTime is when a finished leg was played.
type LegTime struct {
	Set int `json:"set"`
	Leg int `json:"leg"` // in the set
	Span
}

// LegTime returns when a leg was played, the zero Span while it isn't finished.
func (s State) LegTime(set, leg int) Span {
	for _, l := range s.Legs {
		if l.Set == set && l.Leg == leg {
			return l.Span
		}
	}
	return Span{}
}

// SetTime returns when a set (starting at 1) was played, the zero Span while it isn't
// finished.
func (s State) SetTime(set int) Span {
	if set < 1 || set > len(s.Sets) {
		return Span{}
	}
	return s.Sets[set-1]
}

// Common returns the state shared by all modes, also from a mode's state embedding it.
func (s State) Common() State {
	return s
}

// State returns the state of the order of play, with the ids of the players.
func (t *Turn) State(playerIDs []int64) State {
	id := func(i int) *int64 {
		id := playerIDs[i]
		return &id
	}
	s := State{
		Round:             t.Round,
		DartsInTurn:       t.Dart,
		Set:               t.Set,
		Leg:               t.Leg,
		WaitingForTakeout: t.Waiting(),
		LegOver:           t.waiting == nextLeg || t.waiting == nextSet,
		SetOver:           t.waiting == nextSet,
		BullingOff:        t.BullingOff(),
		BullOff:           []BullOffDart{},
		Throwers:          make([]int64, len(t.Throwers)),
		Time:              t.Time,
		Legs:              append([]LegTime{}, t.Legs...),
		Sets:              append([]Span{}, t.Sets...),
	}
	if t.legWinner != nil {
		s.LegWinner = id(*t.legWinner)
	}
	if t.setWinner != nil {
		s.SetWinner = id(*t.setWinner)
	}
	if t.over {
		s.GameOver, s.WinnerID = true, id(t.Player)
	} else {
		s.CurrentPlayerID = id(t.Player)
	}
	for i, p := range t.Throwers {
		if p >= 0 {
			s.Throwers[i] = playerIDs[p]
		}
	}
	if t.bull != nil {
		for _, b := range t.bull.throws {
			s.BullOff = append(s.BullOff, BullOffDart{Entry: b.Entry, PlayerID: playerIDs[b.Player], Round: b.Round})
		}
		if t.bull.tied() {
			for _, p := range t.bull.contenders {
				s.BullOffTie = append(s.BullOffTie, playerIDs[p])
			}
		}
	}
	return s
}
