package x01

import (
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type OutMode uint8

const (
	_             OutMode = iota
	OutModeSingle         // any dart may finish
	OutModeDouble         // the finishing dart must be a double (including bullseye)
	OutModeMaster         // the finishing dart must be a double or a triple
)

var outModeNames = map[OutMode]string{
	OutModeSingle: "single",
	OutModeDouble: "double",
	OutModeMaster: "master",
}

func (m OutMode) MarshalText() ([]byte, error) {
	name, ok := outModeNames[m]
	if !ok {
		return nil, fmt.Errorf("unknown out mode %d", m)
	}
	return []byte(name), nil
}

func (m *OutMode) UnmarshalText(text []byte) error {
	for mode, name := range outModeNames {
		if name == string(text) {
			*m = mode
			return nil
		}
	}
	return fmt.Errorf("unknown out mode %q, use single, double or master", text)
}

// minFinish is the lowest score that can still be finished.
func (m OutMode) minFinish() uint64 {
	if m == OutModeSingle {
		return 1
	}
	return 2
}

func (m OutMode) finishes(d throw.Dart) bool {
	switch m {
	case OutModeDouble:
		return d.Multiplier == 2
	case OutModeMaster:
		return d.Multiplier >= 2
	default:
		return true
	}
}

type Settings struct {
	StartScore uint64  `json:"start_score"`
	OutMode    OutMode `json:"out_mode"`
}

func (s Settings) Validate() error {
	if _, ok := outModeNames[s.OutMode]; !ok {
		return errors.New("out_mode must be single, double or master")
	}
	if s.StartScore < s.OutMode.minFinish() {
		return fmt.Errorf("start_score must be at least %d", s.OutMode.minFinish())
	}
	return nil
}

type PlayerState struct {
	PlayerID  int64  `json:"player_id"`
	Remaining uint64 `json:"remaining"`
}

type State struct {
	Players         []PlayerState `json:"players"`
	CurrentPlayerID *int64        `json:"current_player_id"` // nil once the game is over
	Round           int           `json:"round"`
	DartsInTurn     int           `json:"darts_in_turn"`
	GameOver        bool          `json:"game_over"`
	WinnerIDs       []int64       `json:"winner_ids"` // more than one on a draw
}

// Play replays the darts of a game and returns the resulting state.
// A bust resets the player to the score at the start of the turn and ends the turn.
// A player who reaches exactly 0 has finished; the game ends after that round,
// see package turn.
func Play(s Settings, playerIDs []int64, darts []throw.Dart) (State, error) {
	st := State{}
	for _, id := range playerIDs {
		st.Players = append(st.Players, PlayerState{PlayerID: id, Remaining: s.StartScore})
	}

	t := turn.New()
	var turnStart uint64

	for i, d := range darts {
		if t.Over() {
			return State{}, fmt.Errorf("dart %d: %w", i+1, turn.ErrGameOver)
		}
		if err := d.Validate(); err != nil {
			return State{}, fmt.Errorf("dart %d: %w", i+1, err)
		}

		p := &st.Players[t.Player]
		if t.Dart == 0 {
			turnStart = p.Remaining
		}

		score := d.Score()
		bust := score > p.Remaining ||
			(score == p.Remaining && !s.OutMode.finishes(d)) ||
			(score < p.Remaining && p.Remaining-score < s.OutMode.minFinish())
		if bust {
			p.Remaining = turnStart
			t.End(len(st.Players))
			continue
		}

		p.Remaining -= score
		if p.Remaining == 0 {
			t.Finish(len(st.Players))
			continue
		}
		t.Advance(len(st.Players))
	}

	st.Round = t.Round
	st.DartsInTurn = t.Dart
	st.GameOver = t.Over()
	st.WinnerIDs = []int64{}
	for _, i := range t.Winners() {
		st.WinnerIDs = append(st.WinnerIDs, st.Players[i].PlayerID)
	}
	if !t.Over() {
		st.CurrentPlayerID = &st.Players[t.Player].PlayerID
	}
	return st, nil
}
