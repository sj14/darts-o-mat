// Package stats sums up the statistics of a player over many games.
package stats

import (
	"math"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
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
	// Running games and tournaments of the mode the player left themselves (Quit).
	GamesLeft       int `json:"games_left"`
	TournamentsLeft int `json:"tournaments_left"`
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
	// Reputation is the thumbs the player got from others after games.
	Reputation player.Reputation `json:"reputation"`
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

// FormGames is how many of a player's most recent completed games of a mode their form is
// worked out from, so it shows how they play now; MinFormGames is how many it needs.
const (
	FormGames    = 20
	MinFormGames = 3
)

// Form is how well a player plays a mode lately, to find opponents of about the same
// strength: the three-dart average in X01, the share of hits in around the clock. Both
// hardly depend on the game's settings (start score, check-out, bull, multiplier skips),
// unlike e.g. the darts to finish. results are the player's of the mode, the most recent
// FormGames of them; ok is false with fewer than MinFormGames.
func Form(mode game.Mode, results []Result) (form float64, ok bool) {
	if len(results) < MinFormGames {
		return 0, false
	}
	var s Summary
	for _, r := range results {
		s.AddResult(r)
	}
	switch mode {
	case game.ModeX01:
		return s.X01.Average, true
	case game.ModeAroundTheClock:
		return s.AroundTheClock.HitRate, true
	}
	return 0, false
}

// How far apart two forms may be (FormWait): FormStep of the mode at first, one more step
// every FormWiden someone waited for an opponent, anyone after FormAnyone.
const (
	FormWiden  = 30 * time.Second
	FormAnyone = 3 * time.Minute
)

// FormStep is how much two forms of the mode may differ at first, and how much more with
// every FormWiden of waiting: 5 points of the X01 average, 5 percentage points of the
// around the clock hit rate.
func FormStep(mode game.Mode) float64 {
	if mode == game.ModeAroundTheClock {
		return 0.05
	}
	return 5
}

// FormWait is how long one side has to have waited for an opponent until two forms of the
// mode fit to play each other: 0 within one step, one FormWiden more for every further
// step, at most FormAnyone. As forms don't change while someone waits, the moment two fit
// is known in advance.
func FormWait(mode game.Mode, a, b float64) time.Duration {
	// A little slack, so 0.45 and 0.40 are one step of 0.05 despite rounding.
	steps := math.Ceil(math.Abs(a-b)/FormStep(mode) - 1e-9)
	if steps <= 1 {
		return 0
	}
	return min(time.Duration(steps-1)*FormWiden, FormAnyone)
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

// Quit is a running game or tournament a player left on their own (not because the time
// for their turns ran out, or their account was deleted), counted for them alone and not
// for the others in it. Leaving a game of a tournament counts as leaving the tournament,
// leaving a lobby doesn't count. Stored in the table quits when it happens.
type Quit struct {
	bun.BaseModel `bun:"table:quits"`

	PlayerID   int64     `bun:",pk"`
	Tournament bool      `bun:",pk"` // TargetID is a tournament's, else a game's
	TargetID   int64     `bun:",pk"`
	Mode       game.Mode `bun:",notnull"`
}

// AddQuit adds a game or tournament the player left to the summary.
func (s *Summary) AddQuit(q Quit) {
	r := &s.X01.Record
	if q.Mode == game.ModeAroundTheClock {
		r = &s.AroundTheClock.Record
	}
	if q.Tournament {
		r.TournamentsLeft++
	} else {
		r.GamesLeft++
	}
}

// Any reports whether there is anything to show for the mode: a game played, or a game
// or tournament left.
func (r Record) Any() bool {
	return r.Games > 0 || r.GamesLeft > 0 || r.TournamentsLeft > 0
}
