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
	Wins            int     `json:"wins"`
	Legs            int     `json:"legs"` // legs won
	Sets            int     `json:"sets"` // sets won
	Darts           int     `json:"darts"`
	Points          uint64  `json:"points"`
	Average         float64 `json:"average"` // points per three darts
	HighestTurn     uint64  `json:"highest_turn"`
	Turns100        int     `json:"turns_100"`
	Turns140        int     `json:"turns_140"`
	Turns180        int     `json:"turns_180"`
	HighestCheckout uint64  `json:"highest_checkout"`
	BestLeg         int     `json:"best_leg"` // fewest darts in a won leg, 0 without one
	// Finished tournaments of the mode the player took part in, and won.
	Tournaments    int `json:"tournaments"`
	TournamentWins int `json:"tournament_wins"`
}

type AroundTheClock struct {
	Games   int     `json:"games"`
	Wins    int     `json:"wins"`
	Legs    int     `json:"legs"` // legs won
	Sets    int     `json:"sets"` // sets won
	Darts   int     `json:"darts"`
	Hits    int     `json:"hits"`
	HitRate float64 `json:"hit_rate"`
	BestLeg int     `json:"best_leg"` // fewest darts in a won leg, 0 without one
	// Finished tournaments of the mode the player took part in, and won.
	Tournaments    int `json:"tournaments"`
	TournamentWins int `json:"tournament_wins"`
}

// Summary of a player over all finished games.
type Summary struct {
	Player         player.Player  `json:"player"`
	X01            X01            `json:"x01"`
	AroundTheClock AroundTheClock `json:"around_the_clock"`
}

// Add adds a game with its state (see game.Game.Play) to the summary.
// Games that are not over or that the player didn't take part in are skipped.
func (s *Summary) Add(state any) {
	switch st := state.(type) {
	case x01.State:
		i := slices.IndexFunc(st.Players, func(p x01.PlayerState) bool { return p.PlayerID == s.Player.ID })
		if !st.GameOver || i < 0 {
			return
		}
		s.X01.add(st.Players[i], *st.WinnerID == s.Player.ID)
	case aroundtheclock.State:
		i := slices.IndexFunc(st.Players, func(p aroundtheclock.PlayerState) bool { return p.PlayerID == s.Player.ID })
		if !st.GameOver || i < 0 {
			return
		}
		s.AroundTheClock.add(st.Players[i], *st.WinnerID == s.Player.ID)
	}
}

func (s *X01) add(p x01.PlayerState, won bool) {
	s.Games++
	s.Legs += p.Legs
	s.Sets += p.Sets
	s.Darts += p.Stats.Darts
	s.Points += p.Stats.Points
	if s.Darts > 0 {
		s.Average = float64(s.Points) * 3 / float64(s.Darts)
	}
	s.HighestTurn = max(s.HighestTurn, p.Stats.HighestTurn)
	s.Turns100 += p.Stats.Turns100
	s.Turns140 += p.Stats.Turns140
	s.Turns180 += p.Stats.Turns180
	s.HighestCheckout = max(s.HighestCheckout, p.Stats.Checkout)
	s.BestLeg = best(s.BestLeg, p.Stats.BestLeg)
	if won {
		s.Wins++
	}
}

func (s *AroundTheClock) add(p aroundtheclock.PlayerState, won bool) {
	s.Games++
	s.Legs += p.Legs
	s.Sets += p.Sets
	s.Darts += p.Stats.Darts
	s.Hits += p.Stats.Hits
	if s.Darts > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Darts)
	}
	s.BestLeg = best(s.BestLeg, p.Stats.BestLeg)
	if won {
		s.Wins++
	}
}

// AddTournament adds a finished tournament of the mode the player took part in.
func (s *Summary) AddTournament(mode game.Mode, won bool) {
	count, wins := &s.X01.Tournaments, &s.X01.TournamentWins
	if mode == game.ModeAroundTheClock {
		count, wins = &s.AroundTheClock.Tournaments, &s.AroundTheClock.TournamentWins
	}
	*count++
	if won {
		*wins++
	}
}

// best returns the fewer darts, where 0 means none.
func best(current, darts int) int {
	if current == 0 || darts == 0 {
		return max(current, darts)
	}
	return min(current, darts)
}
