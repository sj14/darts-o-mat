package aroundtheclock

import (
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type Settings struct {
	// Bull adds the bull as the last target after 20.
	Bull bool `json:"bull"`
	// MultiplierSkips lets a double advance two targets and a triple three.
	// Otherwise every hit in the target segment advances one target.
	MultiplierSkips bool `json:"multiplier_skips"`
}

func (s Settings) Validate() error {
	return nil
}

type PlayerState struct {
	PlayerID int64 `json:"player_id"`
	// Target is the number to hit next in the current leg (1-20, or 25 for the bull),
	// 0 once the player has finished.
	Target uint8 `json:"target"`
	Legs   int   `json:"legs"` // legs won
	Stats  Stats `json:"stats"`
}

// Stats of a player in one game, over all legs.
type Stats struct {
	Darts   int     `json:"darts"`
	Hits    int     `json:"hits"`     // darts that hit the current target
	HitRate float64 `json:"hit_rate"` // hits per dart, 0 to 1
	BestLeg int     `json:"best_leg"` // fewest darts in a won leg, 0 without one

	legDarts int // darts in the current leg
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
	PlayerID int64        `json:"player_id"`
	Leg      int          `json:"leg"`
	Round    int          `json:"round"`
	Darts    []throw.Dart `json:"darts"`
	Hits     []bool       `json:"hits"`     // per dart: whether it hit the target at that moment
	From     uint8        `json:"from"`     // target at the start of the turn
	Target   uint8        `json:"target"`   // after this turn, 0 once finished
	Complete bool         `json:"complete"` // false while the player still throws
}

// Play replays the darts of a game and returns the resulting state.
// The players hit 1 to 20 in order, optionally followed by the bull.
// A player who hits the last target wins the leg, see package turn.
func Play(s Settings, playerIDs []int64, darts []throw.Dart) (State, error) {
	return PlayLog(s, playerIDs, turn.Log{Entries: darts})
}

// PlayLog replays a game's log of darts and takeouts, see Play.
func PlayLog(s Settings, playerIDs []int64, log turn.Log) (State, error) {
	st := State{Turns: []Turn{}}
	for _, id := range playerIDs {
		st.Players = append(st.Players, PlayerState{PlayerID: id, Target: 1})
	}

	t := log.Turn(len(st.Players))
	t.OnLegStart = func() {
		for i := range st.Players {
			st.Players[i].Target = 1
			st.Players[i].Stats.legDarts = 0
		}
	}
	var cur *Turn // the turn being thrown

	st.Throwers = make([]int64, len(log.Entries))
	err := log.Replay(t, func(i int, d throw.Dart) error {
		st.Throwers[i] = st.Players[t.Player].PlayerID
		if err := d.Validate(); err != nil {
			return fmt.Errorf("dart %d: %w", i+1, err)
		}

		p := &st.Players[t.Player]
		if t.Dart == 0 {
			st.Turns = append(st.Turns, Turn{PlayerID: p.PlayerID, Leg: t.Leg, Round: t.Round, From: p.Target})
			cur = &st.Turns[len(st.Turns)-1]
		}
		p.Stats.Darts++
		p.Stats.legDarts++
		hit := !d.Miss && d.Number == p.Target
		cur.Darts, cur.Hits = append(cur.Darts, d), append(cur.Hits, hit)
		if hit {
			p.Stats.Hits++
			p.Target = s.next(p.Target, d.Multiplier)
		}
		cur.Target = p.Target
		if p.Target == 0 {
			cur.Complete = true
			p.Stats.BestLeg = fewest(p.Stats.BestLeg, p.Stats.legDarts)
			t.Finish()
			return nil
		}
		t.Advance()
		cur.Complete = t.Dart == 0 || t.Waiting()
		return nil
	})
	if err != nil {
		return State{}, err
	}

	for i := range st.Players {
		p := &st.Players[i]
		p.Legs = t.LegsWon[i]
		if p.Stats.Darts > 0 {
			p.Stats.HitRate = float64(p.Stats.Hits) / float64(p.Stats.Darts)
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

// next returns the target after hitting target, 0 if the player has finished.
func (s Settings) next(target, multiplier uint8) uint8 {
	if target == throw.Bull {
		return 0
	}

	steps := uint8(1)
	if s.MultiplierSkips {
		steps = multiplier
	}

	if target+steps <= 20 {
		return target + steps
	}
	if s.Bull {
		return throw.Bull
	}
	return 0
}

// fewest returns the fewer darts, where 0 means none yet.
func fewest(current, darts int) int {
	if current == 0 {
		return darts
	}
	return min(current, darts)
}
