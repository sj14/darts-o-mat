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
	Target  uint8 `json:"target"`
	Legs    int   `json:"legs"`     // legs won, over the whole game
	SetLegs int   `json:"set_legs"` // legs won in the current set (all legs without sets)
	Sets    int   `json:"sets"`     // sets won
	Stats   Stats `json:"stats"`
}

// Stats of a player in one game, over all legs.
type Stats struct {
	Darts   int     `json:"darts"`
	Hits    int     `json:"hits"`     // darts that hit the current target
	HitRate float64 `json:"hit_rate"` // hits per dart, 0 to 1
	BestLeg int     `json:"best_leg"` // fewest darts in a won leg, 0 without one
}

type State struct {
	turn.State
	Players []PlayerState `json:"players"`
	// Turns lists every turn in the order they were thrown; the last one may still be in progress.
	Turns []Turn `json:"turns"`
}

// Turn is one player's turn of up to three darts.
type Turn struct {
	PlayerID int64        `json:"player_id"`
	Set      int          `json:"set"`
	Leg      int          `json:"leg"` // in the set
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
		}
	}
	var cur *Turn // the turn being thrown

	err := log.Replay(t, func(i int, d throw.Dart) error {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("dart %d: %w", i+1, err)
		}

		p := &st.Players[t.Player]
		if t.Dart == 0 {
			st.Turns = append(st.Turns, Turn{PlayerID: p.PlayerID, Set: t.Set, Leg: t.Leg, Round: t.Round, From: p.Target})
			cur = &st.Turns[len(st.Turns)-1]
		}
		p.Stats.Darts++
		hit := d.Segment != throw.Miss && d.Number == p.Target
		cur.Darts, cur.Hits = append(cur.Darts, d), append(cur.Hits, hit)
		if hit {
			p.Stats.Hits++
			p.Target = s.next(p.Target, d.Segment.Multiplier())
		}
		cur.Target = p.Target
		if p.Target == 0 {
			cur.Complete = true
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
		p.Legs, p.SetLegs, p.Sets, p.Stats.BestLeg = t.LegsWon[i], t.SetLegs[i], t.SetsWon[i], t.BestLeg[i]
		if p.Stats.Darts > 0 {
			p.Stats.HitRate = float64(p.Stats.Hits) / float64(p.Stats.Darts)
		}
	}

	st.State = t.State(playerIDs)
	return st, nil
}

// next returns the target after hitting target, 0 if the player has finished.
func (s Settings) next(target uint8, multiplier int) uint8 {
	if target == throw.BullNumber {
		return 0
	}

	steps := uint8(1)
	if s.MultiplierSkips {
		steps = uint8(multiplier)
	}

	if target+steps <= 20 {
		return target + steps
	}
	if s.Bull {
		return throw.BullNumber
	}
	return 0
}
