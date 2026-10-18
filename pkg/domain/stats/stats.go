// Package stats sums up the statistics of a player over many games.
package stats

import (
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/uptrace/bun"
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
	// Averages of the games' and legs' durations (see turn.Times), zero without timed ones.
	AvgGame time.Duration `json:"avg_game"`
	AvgLeg  time.Duration `json:"avg_leg"`

	gameTime, legTime     time.Duration
	timedGames, timedLegs int
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
	// Heat is where the player's darts hit the board; empty without camera darts.
	Heat Heat `json:"heat"`
}

// Result is what one player got out of one game that is over: the building block of a
// Summary. It is stored (table game_stats) once the game is completed, because then
// nothing can change anymore; the statistics add up the results instead of replaying
// every game.
type Result struct {
	bun.BaseModel `bun:"table:game_stats"`

	GameID   int64     `bun:",pk"`
	PlayerID int64     `bun:",pk"`
	Mode     game.Mode `bun:",notnull"`
	Won      bool      `bun:",notnull"`
	Legs     int       // legs won
	Sets     int       // sets won
	BestLeg  int       // fewest darts in a won leg, 0 without one
	// GameTime is the duration of the game (0 without times), LegTime the sum of the
	// durations of its timed legs.
	GameTime  time.Duration
	LegTime   time.Duration
	TimedLegs int
	Darts     int
	// X01: points scored, the turns by their points and the highest finish.
	Points      uint64
	HighestTurn uint64
	Turns100    int
	Turns140    int
	Turns180    int
	Checkout    uint64
	// Around the clock: darts that hit the target.
	Hits int
}

// ResultsOf returns the result of every player of a game that is over (see
// game.Game.Play), none for a game that isn't.
func ResultsOf(gameID int64, state game.State) []Result {
	c := state.Common()
	if !c.GameOver {
		return nil
	}
	var results []Result
	for id := range c.Records {
		if r, ok := resultOf(state, id); ok {
			r.GameID = gameID
			results = append(results, r)
		}
	}
	return results
}

// resultOf is the result of one player, if the game is over and they took part.
func resultOf(state game.State, playerID int64) (Result, bool) {
	c := state.Common()
	rec, ok := c.Records[playerID]
	if !c.GameOver || !ok {
		return Result{}, false
	}
	r := Result{
		PlayerID: playerID,
		Won:      *c.WinnerID == playerID,
		Legs:     rec.Legs,
		Sets:     rec.Sets,
		BestLeg:  rec.BestLeg,
		GameTime: max(c.Times.Game.Duration(), 0),
	}
	for _, l := range c.Times.Legs {
		if d := l.Duration(); d > 0 {
			r.LegTime += d
			r.TimedLegs++
		}
	}
	switch st := state.(type) {
	case x01.State:
		p, _ := st.Player(playerID)
		r.Mode = game.ModeX01
		r.Darts, r.Points = p.Stats.Darts, p.Stats.Points
		r.HighestTurn, r.Checkout = p.Stats.HighestTurn, p.Stats.Checkout
		r.Turns100, r.Turns140, r.Turns180 = p.Stats.Turns100, p.Stats.Turns140, p.Stats.Turns180
	case aroundtheclock.State:
		p, _ := st.Player(playerID)
		r.Mode = game.ModeAroundTheClock
		r.Darts, r.Hits = p.Stats.Darts, p.Stats.Hits
	default:
		return Result{}, false
	}
	return r, true
}

// Add adds a game with its state (see game.Game.Play) to the summary.
// Games that are not over or that the player didn't take part in are skipped.
func (s *Summary) Add(state game.State) {
	if r, ok := resultOf(state, s.Player.ID); ok {
		s.AddResult(r)
	}
}

// AddResult adds the result of one game of the player to the summary.
func (s *Summary) AddResult(r Result) {
	switch r.Mode {
	case game.ModeX01:
		s.X01.Record.add(r)
		s.X01.add(r)
	case game.ModeAroundTheClock:
		s.AroundTheClock.Record.add(r)
		s.AroundTheClock.add(r)
	}
}

func (r *Record) add(res Result) {
	r.Games++
	r.Legs += res.Legs
	r.Sets += res.Sets
	r.BestLeg = best(r.BestLeg, res.BestLeg)
	if res.Won {
		r.Wins++
	}
	if res.GameTime > 0 {
		r.gameTime += res.GameTime
		r.timedGames++
		r.AvgGame = r.gameTime / time.Duration(r.timedGames)
	}
	r.legTime += res.LegTime
	r.timedLegs += res.TimedLegs
	if r.timedLegs > 0 {
		r.AvgLeg = r.legTime / time.Duration(r.timedLegs)
	}
}

func (s *X01) add(p Result) {
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

func (s *AroundTheClock) add(p Result) {
	s.Darts += p.Darts
	s.Hits += p.Hits
	if s.Darts > 0 {
		s.HitRate = float64(s.Hits) / float64(s.Darts)
	}
}

// TournamentResult is what one player got out of a tournament that is over: stored like
// Result (table tournament_stats), once the last match is completed.
type TournamentResult struct {
	bun.BaseModel `bun:"table:tournament_stats"`

	TournamentID int64     `bun:",pk"`
	PlayerID     int64     `bun:",pk"`
	Mode         game.Mode `bun:",notnull"`
	Won          bool      `bun:",notnull"`
}

// AddTournamentResult adds a finished tournament of the player to the summary.
func (s *Summary) AddTournamentResult(t TournamentResult) {
	r := &s.X01.Record
	if t.Mode == game.ModeAroundTheClock {
		r = &s.AroundTheClock.Record
	}
	r.Tournaments++
	if t.Won {
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
