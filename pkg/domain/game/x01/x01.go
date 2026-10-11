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
	Remaining uint64 `json:"remaining"` // in the current leg
	Legs      int    `json:"legs"`      // legs won
	Stats     Stats  `json:"stats"`
}

// Stats of a player in one game, over all legs. Turns count once they are over:
// after the third dart, a bust or the checkout.
type Stats struct {
	Darts       int     `json:"darts"`
	Points      uint64  `json:"points"`  // scored; busted turns score 0
	Average     float64 `json:"average"` // points per three darts
	HighestTurn uint64  `json:"highest_turn"`
	Turns100    int     `json:"turns_100"` // turns scoring 100 to 139
	Turns140    int     `json:"turns_140"` // turns scoring 140 to 179
	Turns180    int     `json:"turns_180"`
	Checkout    uint64  `json:"checkout"` // points of the highest finishing turn, 0 until finished
	BestLeg     int     `json:"best_leg"` // fewest darts in a won leg, 0 without one

	legDarts int // darts in the current leg
}

func (s *Stats) addTurn(points uint64) {
	s.HighestTurn = max(s.HighestTurn, points)
	switch {
	case points == 180:
		s.Turns180++
	case points >= 140:
		s.Turns140++
	case points >= 100:
		s.Turns100++
	}
}

type State struct {
	Players         []PlayerState `json:"players"`
	CurrentPlayerID *int64        `json:"current_player_id"` // nil once the game is over
	Round           int           `json:"round"`
	DartsInTurn     int           `json:"darts_in_turn"`
	Leg             int           `json:"leg"` // starts at 1
	GameOver        bool          `json:"game_over"`
	WinnerID        *int64        `json:"winner_id"` // nil until the game is over
	// LegOver is set while waiting for the takeout after a leg was won.
	LegOver bool `json:"leg_over"`
	// WaitingForTakeout is set when the turn is over, but the darts weren't pulled out yet
	// (only in games played on a board, see turn.Log.WaitForTakeout).
	WaitingForTakeout bool `json:"waiting_for_takeout"`
	// Throwers is the player who threw each entry of the log, 0 for takeouts.
	Throwers []int64 `json:"-"`
	// Turns lists every turn in the order they were thrown; the last one may still be in progress.
	Turns []Turn `json:"turns"`
}

// Turn is one player's turn of up to three darts.
type Turn struct {
	PlayerID  int64        `json:"player_id"`
	Leg       int          `json:"leg"`
	Round     int          `json:"round"`
	Darts     []throw.Dart `json:"darts"`
	Points    uint64       `json:"points"`    // scored in this turn, 0 when busted
	Remaining uint64       `json:"remaining"` // after this turn
	Bust      bool         `json:"bust"`
	Checkout  bool         `json:"checkout"`
	Complete  bool         `json:"complete"` // false while the player still throws
}

// Play replays the darts of a game and returns the resulting state.
// A bust resets the player to the score at the start of the turn and ends the turn.
// A player who reaches exactly 0 wins the leg, see package turn.
func Play(s Settings, playerIDs []int64, darts []throw.Dart) (State, error) {
	return PlayLog(s, playerIDs, turn.Log{Entries: darts})
}

// PlayLog replays a game's log of darts and takeouts, see Play.
func PlayLog(s Settings, playerIDs []int64, log turn.Log) (State, error) {
	st := State{Turns: []Turn{}}
	for _, id := range playerIDs {
		st.Players = append(st.Players, PlayerState{PlayerID: id, Remaining: s.StartScore})
	}

	t := log.Turn(len(st.Players))
	t.OnLegStart = func() {
		for i := range st.Players {
			st.Players[i].Remaining = s.StartScore
			st.Players[i].Stats.legDarts = 0
		}
	}
	var turnStart uint64
	var cur *Turn // the turn being thrown

	st.Throwers = make([]int64, len(log.Entries))
	err := log.Replay(t, func(i int, d throw.Dart) error {
		st.Throwers[i] = st.Players[t.Player].PlayerID
		if err := d.Validate(); err != nil {
			return fmt.Errorf("dart %d: %w", i+1, err)
		}

		p := &st.Players[t.Player]
		if t.Dart == 0 {
			turnStart = p.Remaining
			st.Turns = append(st.Turns, Turn{PlayerID: p.PlayerID, Leg: t.Leg, Round: t.Round})
			cur = &st.Turns[len(st.Turns)-1]
		}
		p.Stats.Darts++
		p.Stats.legDarts++
		cur.Darts = append(cur.Darts, d)

		score := d.Score()
		bust := score > p.Remaining ||
			(score == p.Remaining && !s.OutMode.finishes(d)) ||
			(score < p.Remaining && p.Remaining-score < s.OutMode.minFinish())
		if bust {
			p.Stats.Points -= turnStart - p.Remaining // the turn's earlier darts don't count
			p.Remaining = turnStart
			p.Stats.addTurn(0)
			cur.Points, cur.Remaining, cur.Bust, cur.Complete = 0, turnStart, true, true
			t.End()
			return nil
		}

		p.Remaining -= score
		p.Stats.Points += score
		cur.Points, cur.Remaining = turnStart-p.Remaining, p.Remaining
		if p.Remaining == 0 {
			p.Stats.addTurn(turnStart)
			p.Stats.Checkout = max(p.Stats.Checkout, turnStart)
			p.Stats.BestLeg = fewest(p.Stats.BestLeg, p.Stats.legDarts)
			cur.Checkout, cur.Complete = true, true
			t.Finish()
			return nil
		}
		t.Advance()
		// The turn is over after the third dart: the next player throws, or on a board
		// everyone waits for the takeout.
		if t.Dart == 0 || t.Waiting() {
			p.Stats.addTurn(turnStart - p.Remaining)
			cur.Complete = true
		}
		return nil
	})
	if err != nil {
		return State{}, err
	}

	for i := range st.Players {
		p := &st.Players[i]
		p.Legs = t.LegsWon[i]
		if p.Stats.Darts > 0 {
			p.Stats.Average = float64(p.Stats.Points) * 3 / float64(p.Stats.Darts)
		}
	}

	st.Round, st.Leg = t.Round, t.Leg
	st.DartsInTurn = t.Dart
	st.WaitingForTakeout, st.LegOver = t.Waiting(), t.LegOver()
	if w, over := t.Winner(); over {
		st.GameOver, st.WinnerID = true, &st.Players[w].PlayerID
	} else {
		st.CurrentPlayerID = &st.Players[t.Player].PlayerID
	}
	return st, nil
}

// fewest returns the fewer darts, where 0 means none yet.
func fewest(current, darts int) int {
	if current == 0 {
		return darts
	}
	return min(current, darts)
}
