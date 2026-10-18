package tournament

import (
	"cmp"
	"math/bits"
	"slices"
)

// Result is what became of a match's game.
type Result struct {
	GameID int64 `json:"game_id"`
	// Over is set once the game has a winner and its result is final (the game is
	// completed); only then the winner goes on. Until then it is running or Unconfirmed.
	Over bool `json:"over"`
	// Unconfirmed is set when the game has a winner but the players can still correct it.
	Unconfirmed bool  `json:"unconfirmed,omitempty"`
	WinnerID    int64 `json:"winner_id,omitempty"`
	// Score is what each player of the match won: sets, or legs in a game without sets.
	Score [2]int `json:"score"`
}

// Match is a game of the schedule. Its players are known once the matches before it
// are decided (knockout); 0 stands for a player not known yet.
type Match struct {
	Index     int      `json:"index"`
	Round     int      `json:"round"` // starts at 1
	PlayerIDs [2]int64 `json:"player_ids"`
	// Bye is set for a first-round match with only one player: they go through
	// without a game.
	Bye    bool    `json:"bye,omitempty"`
	Result *Result `json:"result,omitempty"` // nil until its game was started
}

// Ready reports whether the match can be played: both players are known and it has
// no game yet that is running or decided.
func (m Match) Ready() bool {
	return !m.Bye && m.PlayerIDs[0] != 0 && m.PlayerIDs[1] != 0 && m.Result == nil
}

// Winner returns the player who won the match, 0 while it isn't decided.
func (m Match) Winner() int64 {
	switch {
	case m.Bye:
		return max(m.PlayerIDs[0], m.PlayerIDs[1])
	case m.Result != nil && m.Result.Over:
		return m.Result.WinnerID
	}
	return 0
}

// Standing is a player's line in the table of a round-robin tournament.
type Standing struct {
	PlayerID int64 `json:"player_id"`
	Played   int   `json:"played"`
	Won      int   `json:"won"`
	Lost     int   `json:"lost"`
	// For and Against are the sets (legs without sets) won and lost in all matches.
	For     int `json:"for"`
	Against int `json:"against"`
}

// State is the schedule of a tournament with the results so far.
type State struct {
	Rounds  int     `json:"rounds"`
	Matches []Match `json:"matches"` // ordered by round
	// Standings is the table of a round-robin tournament, best first: most matches won,
	// then the difference of sets (legs) won and lost, then more won, then the draw.
	Standings []Standing `json:"standings,omitempty"`
	Over      bool       `json:"over"`
	WinnerID  *int64     `json:"winner_id,omitempty"`
}

// Plan computes the schedule of a tournament with the given format and players (in
// the order of the draw) and the results of the matches started so far, by index.
func Plan(format Format, playerIDs []int64, results map[int]Result) State {
	if format == Knockout {
		return knockout(playerIDs, results)
	}
	return roundRobin(playerIDs, results)
}

// knockout builds the bracket: the seeds (the draw) are placed so that the best ones
// meet as late as possible, and the byes go to the best seeds.
func knockout(playerIDs []int64, results map[int]Result) State {
	size := 1 << bits.Len(uint(len(playerIDs)-1)) // the next power of two
	st := State{Rounds: bits.Len(uint(size)) - 1}

	// Round 1 from the seeding.
	slots := seeding(size)
	var round []int // indexes of the matches of the previous round
	for i := 0; i < size; i += 2 {
		m := Match{Index: len(st.Matches), Round: 1}
		for j, seed := range slots[i : i+2] {
			if seed <= len(playerIDs) {
				m.PlayerIDs[j] = playerIDs[seed-1]
			}
		}
		m.Bye = m.PlayerIDs[0] == 0 || m.PlayerIDs[1] == 0
		m.addResult(results)
		round = append(round, m.Index)
		st.Matches = append(st.Matches, m)
	}
	// Each further round pairs the winners of neighbouring matches.
	for r := 2; r <= st.Rounds; r++ {
		var next []int
		for i := 0; i < len(round); i += 2 {
			m := Match{Index: len(st.Matches), Round: r}
			for j, prev := range round[i : i+2] {
				m.PlayerIDs[j] = st.Matches[prev].Winner()
			}
			m.addResult(results)
			next = append(next, m.Index)
			st.Matches = append(st.Matches, m)
		}
		round = next
	}
	if winner := st.Matches[len(st.Matches)-1].Winner(); winner != 0 {
		st.Over, st.WinnerID = true, &winner
	}
	return st
}

// seeding returns the seeds (1 = best) in the order of the bracket's slots, so that
// seed 1 and 2 can only meet in the final: for 8, 1 8 4 5 2 7 3 6.
func seeding(size int) []int {
	slots := []int{1}
	for n := 2; n <= size; n *= 2 {
		var next []int
		for _, s := range slots {
			next = append(next, s, n+1-s)
		}
		slots = next
	}
	return slots
}

// roundRobin schedules everyone against everyone with the circle method: one player
// stays, the others turn around them, so everyone plays once per round. With an odd
// number of players one sits out each round.
func roundRobin(playerIDs []int64, results map[int]Result) State {
	ids := slices.Clone(playerIDs)
	if len(ids)%2 == 1 {
		ids = append(ids, 0) // sits out
	}
	n := len(ids)
	st := State{Rounds: n - 1}
	for r := 1; r <= st.Rounds; r++ {
		for i := range n / 2 {
			a, b := ids[i], ids[n-1-i]
			if a == 0 || b == 0 {
				continue
			}
			if r%2 == 0 && i == 0 {
				a, b = b, a // the fixed player doesn't always throw first
			}
			m := Match{Index: len(st.Matches), Round: r, PlayerIDs: [2]int64{a, b}}
			m.addResult(results)
			st.Matches = append(st.Matches, m)
		}
		// Turn everyone but the first by one.
		ids = append(ids[:1], append([]int64{ids[n-1]}, ids[1:n-1]...)...)
	}

	// The table, in the order of the draw for ties.
	st.Standings = make([]Standing, len(playerIDs))
	pos := map[int64]int{}
	for i, id := range playerIDs {
		st.Standings[i].PlayerID, pos[id] = id, i
	}
	over := true
	for _, m := range st.Matches {
		if m.Winner() == 0 {
			over = false
			continue
		}
		for j, id := range m.PlayerIDs {
			s := &st.Standings[pos[id]]
			s.Played++
			s.For += m.Result.Score[j]
			s.Against += m.Result.Score[1-j]
			if m.Winner() == id {
				s.Won++
			} else {
				s.Lost++
			}
		}
	}
	slices.SortStableFunc(st.Standings, func(a, b Standing) int {
		return cmp.Or(
			cmp.Compare(b.Won, a.Won),
			cmp.Compare(b.For-b.Against, a.For-a.Against),
			cmp.Compare(b.For, a.For),
		)
	})
	if over {
		st.Over, st.WinnerID = true, &st.Standings[0].PlayerID
	}
	return st
}

// addResult adds the result of the match's game, if it was started (only possible
// once both players are known).
func (m *Match) addResult(results map[int]Result) {
	if r, ok := results[m.Index]; ok && !m.Bye && m.PlayerIDs[0] != 0 && m.PlayerIDs[1] != 0 {
		m.Result = &r
	}
}
