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
	// Target is the number to hit next (1-20, or 25 for the bull), 0 once the player has finished.
	Target uint8 `json:"target"`
	Stats  Stats `json:"stats"`
}

// Stats of a player in one game.
type Stats struct {
	Darts   int     `json:"darts"`
	Hits    int     `json:"hits"`     // darts that hit the current target
	HitRate float64 `json:"hit_rate"` // hits per dart, 0 to 1
}

type State struct {
	Players         []PlayerState `json:"players"`
	CurrentPlayerID *int64        `json:"current_player_id"` // nil once the game is over
	Round           int           `json:"round"`
	DartsInTurn     int           `json:"darts_in_turn"`
	GameOver        bool          `json:"game_over"`
	WinnerIDs       []int64       `json:"winner_ids"` // more than one on a draw
	// WaitingForTakeout is set when the turn is over, but the darts weren't pulled out yet
	// (only in games played on a board, see turn.Log.WaitForTakeout).
	WaitingForTakeout bool `json:"waiting_for_takeout"`
}

// Play replays the darts of a game and returns the resulting state.
// The players hit 1 to 20 in order, optionally followed by the bull.
// A player who hits the last target has finished; the game ends after that round,
// see package turn.
func Play(s Settings, playerIDs []int64, darts []throw.Dart) (State, error) {
	return PlayLog(s, playerIDs, turn.Log{Darts: darts})
}

// PlayLog replays a game's log of darts and takeouts, see Play.
func PlayLog(s Settings, playerIDs []int64, log turn.Log) (State, error) {
	st := State{}
	for _, id := range playerIDs {
		st.Players = append(st.Players, PlayerState{PlayerID: id, Target: 1})
	}

	t := turn.New()
	if log.WaitForTakeout {
		t = turn.NewWaitingForTakeouts()
	}

	err := log.Replay(&t, len(st.Players), func(i int, d throw.Dart) error {
		if t.Over() {
			return fmt.Errorf("dart %d: %w", i+1, turn.ErrGameOver)
		}
		if err := d.Validate(); err != nil {
			return fmt.Errorf("dart %d: %w", i+1, err)
		}

		p := &st.Players[t.Player]
		p.Stats.Darts++
		if !d.Miss && d.Number == p.Target {
			p.Stats.Hits++
			p.Target = s.next(p.Target, d.Multiplier)
			if p.Target == 0 {
				t.Finish(len(st.Players))
				return nil
			}
		}
		t.Advance(len(st.Players))
		return nil
	})
	if err != nil {
		return State{}, err
	}

	for i := range st.Players {
		if p := &st.Players[i]; p.Stats.Darts > 0 {
			p.Stats.HitRate = float64(p.Stats.Hits) / float64(p.Stats.Darts)
		}
	}

	st.Round = t.Round
	st.DartsInTurn = t.Dart
	st.WaitingForTakeout = t.Waiting()
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
