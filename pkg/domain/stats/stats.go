// Package stats sums up the statistics of a player over many games.
package stats

import (
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
)

// Record is what all modes count: games, wins, won legs and sets, the best leg and
// tournaments.
type Record struct {
	Games   int `json:"games"`
	Wins    int `json:"wins"`
	Legs    int `json:"legs"`     // legs won
	Sets    int `json:"sets"`     // sets won
	BestLeg int `json:"best_leg"` // fewest darts in a won leg, 0 without one
	// Finished tournaments of the mode the player took part in, and won.
	Tournaments    int `json:"tournaments"`
	TournamentWins int `json:"tournament_wins"`
}

type X01 struct {
	Record
	Darts           int     `json:"darts"`
	Points          uint64  `json:"points"`
	Average         float64 `json:"average"` // points per three darts
	HighestTurn     uint64  `json:"highest_turn"`
	Turns100        int     `json:"turns_100"`
	Turns140        int     `json:"turns_140"`
	Turns180        int     `json:"turns_180"`
	HighestCheckout uint64  `json:"highest_checkout"`
}

type AroundTheClock struct {
	Record
	Darts   int     `json:"darts"`
	Hits    int     `json:"hits"`
	HitRate float64 `json:"hit_rate"`
}

// Summary of a player over all finished games.
type Summary struct {
	Player         player.Player  `json:"player"`
	X01            X01            `json:"x01"`
	AroundTheClock AroundTheClock `json:"around_the_clock"`
}

// Add adds a game with its state (see game.Game.Play) to the summary.
// Games that are not over or that the player didn't take part in are skipped.
func (s *Summary) Add(state game.State) {
	c := state.Common()
	r, ok := c.Records[s.Player.ID]
	if !c.GameOver || !ok {
		return
	}
	won := *c.WinnerID == s.Player.ID
	switch st := state.(type) {
	case x01.State:
		p, _ := st.Player(s.Player.ID)
		s.X01.Record.add(r, won)
		s.X01.add(p.Stats)
	case aroundtheclock.State:
		p, _ := st.Player(s.Player.ID)
		s.AroundTheClock.Record.add(r, won)
		s.AroundTheClock.add(p.Stats)
	}
}

func (r *Record) add(game turn.Record, won bool) {
	r.Games++
	r.Legs += game.Legs
	r.Sets += game.Sets
	r.BestLeg = best(r.BestLeg, game.BestLeg)
	if won {
		r.Wins++
	}
}

func (s *X01) add(p x01.Stats) {
	s.Darts += p.Darts
	s.Points += p.Points
	if s.Darts > 0 {
		s.Average = float64(s.Points) * 3 / float64(s.Darts)
	}
	s.HighestTurn = max(s.HighestTurn, p.HighestTurn)
	s.Turns100 += p.Turns100
	s.Turns140 += p.Turns140
	s.Turns180 += p.Turns180
	s.HighestCheckout = max(s.HighestCheckout, p.Checkout)
}

func (s *AroundTheClock) add(p aroundtheclock.Stats) {
	s.Darts += p.Darts
	s.Hits += p.Hits
	if s.Darts > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Darts)
	}
}

// AddTournament adds a finished tournament of the mode the player took part in.
func (s *Summary) AddTournament(mode game.Mode, won bool) {
	r := &s.X01.Record
	if mode == game.ModeAroundTheClock {
		r = &s.AroundTheClock.Record
	}
	r.Tournaments++
	if won {
		r.TournamentWins++
	}
}

// best returns the fewer darts, where 0 means none.
func best(current, darts int) int {
	if current == 0 || darts == 0 {
		return max(current, darts)
	}
	return min(current, darts)
}
