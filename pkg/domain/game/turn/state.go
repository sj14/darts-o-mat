package turn

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
