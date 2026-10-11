// Package stats sums up the statistics of a player over many games.
package stats

import (
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
)

type X01 struct {
	Games           int     `json:"games"`
	Wins            int     `json:"wins"` // including draws
	Darts           int     `json:"darts"`
	Points          uint64  `json:"points"`
	Average         float64 `json:"average"` // points per three darts
	HighestTurn     uint64  `json:"highest_turn"`
	Turns100        int     `json:"turns_100"`
	Turns140        int     `json:"turns_140"`
	Turns180        int     `json:"turns_180"`
	HighestCheckout uint64  `json:"highest_checkout"`
	BestGame        int     `json:"best_game"` // fewest darts in a won game, 0 without a win
}

type AroundTheClock struct {
	Games    int     `json:"games"`
	Wins     int     `json:"wins"` // including draws
	Darts    int     `json:"darts"`
	Hits     int     `json:"hits"`
	HitRate  float64 `json:"hit_rate"`
	BestGame int     `json:"best_game"` // fewest darts in a won game, 0 without a win
}

// Summary of a player over all finished games.
type Summary struct {
	Player         player.Player  `json:"player"`
	X01            X01            `json:"x01"`
	AroundTheClock AroundTheClock `json:"around_the_clock"`
}

// Add adds a game with its state (see game.Game.Play) to the summary.
// Games that are not over or that the player didn't take part in are skipped.
func (s *Summary) Add(g game.Game, state any) {
	switch st := state.(type) {
	case x01.State:
		i := slices.IndexFunc(st.Players, func(p x01.PlayerState) bool { return p.PlayerID == s.Player.ID })
		if !st.GameOver || i < 0 {
			return
		}
		s.X01.add(g, st.Players[i], slices.Contains(st.WinnerIDs, s.Player.ID))
	case aroundtheclock.State:
		i := slices.IndexFunc(st.Players, func(p aroundtheclock.PlayerState) bool { return p.PlayerID == s.Player.ID })
		if !st.GameOver || i < 0 {
			return
		}
		s.AroundTheClock.add(st.Players[i], slices.Contains(st.WinnerIDs, s.Player.ID))
	}
}

func (s *X01) add(g game.Game, p x01.PlayerState, won bool) {
	s.Games++
	s.Darts += p.Stats.Darts
	s.Points += g.X01.StartScore - p.Remaining
	if s.Darts > 0 {
		s.Average = float64(s.Points) * 3 / float64(s.Darts)
	}
	s.HighestTurn = max(s.HighestTurn, p.Stats.HighestTurn)
	s.Turns100 += p.Stats.Turns100
	s.Turns140 += p.Stats.Turns140
	s.Turns180 += p.Stats.Turns180
	s.HighestCheckout = max(s.HighestCheckout, p.Stats.Checkout)
	if won {
		s.Wins++
		s.BestGame = best(s.BestGame, p.Stats.Darts)
	}
}

func (s *AroundTheClock) add(p aroundtheclock.PlayerState, won bool) {
	s.Games++
	s.Darts += p.Stats.Darts
	s.Hits += p.Stats.Hits
	if s.Darts > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Darts)
	}
	if won {
		s.Wins++
		s.BestGame = best(s.BestGame, p.Stats.Darts)
	}
}

// best returns the fewer darts, where 0 means no game yet.
func best(current, darts int) int {
	if current == 0 {
		return darts
	}
	return min(current, darts)
}
