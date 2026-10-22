package tournament

import (
	"slices"
	"testing"
)

func TestSeeding(t *testing.T) {
	t.Parallel()
	if got, want := seeding(8), []int{1, 8, 4, 5, 2, 7, 3, 6}; !slices.Equal(got, want) {
		t.Errorf("seeding(8) = %v, want %v", got, want)
	}
}

func TestKnockout(t *testing.T) {
	t.Parallel()
	players := []int64{1, 2, 3, 4, 5} // seeds 1 to 5
	st := Plan(Knockout, players, nil)
	if st.Rounds != 3 || len(st.Matches) != 7 {
		t.Fatalf("%d rounds, %d matches, want 3 rounds with 4+2+1 matches", st.Rounds, len(st.Matches))
	}
	// Round 1: the three best seeds have a bye, 4 meets 5.
	want := [][2]int64{{1, 0}, {4, 5}, {2, 0}, {3, 0}}
	for i, w := range want {
		if st.Matches[i].PlayerIDs != w {
			t.Errorf("match %d: %v, want %v", i, st.Matches[i].PlayerIDs, w)
		}
	}
	if !st.Matches[0].Bye || st.Matches[1].Bye || !st.Matches[1].Ready() {
		t.Errorf("byes or ready wrong: %+v", st.Matches[:2])
	}
	// The byes already advanced: 2 meets 3, 1 waits for 4 or 5.
	if st.Matches[4].PlayerIDs != [2]int64{1, 0} || st.Matches[5].PlayerIDs != [2]int64{2, 3} || !st.Matches[5].Ready() {
		t.Fatalf("round 2: %v %v, want [1 0] and [2 3]", st.Matches[4].PlayerIDs, st.Matches[5].PlayerIDs)
	}

	results := map[int]Result{
		1: {GameID: 10, Over: true, WinnerID: 5, Score: [2]int{1, 3}},
		5: {GameID: 11, Over: true, WinnerID: 3, Score: [2]int{2, 3}},
		4: {GameID: 12}, // running
	}
	st = Plan(Knockout, players, results)
	if st.Matches[4].PlayerIDs != [2]int64{1, 5} || st.Matches[4].Ready() {
		t.Errorf("semi-final 1: %v ready %v, want [1 5] running", st.Matches[4].PlayerIDs, st.Matches[4].Ready())
	}
	if st.Matches[6].PlayerIDs != [2]int64{0, 3} || st.Over {
		t.Errorf("final: %v over %v, want [0 3], not over", st.Matches[6].PlayerIDs, st.Over)
	}

	results[4] = Result{GameID: 12, Over: true, WinnerID: 1, Score: [2]int{3, 0}}
	results[6] = Result{GameID: 13, Over: true, WinnerID: 3, Score: [2]int{1, 3}}
	st = Plan(Knockout, players, results)
	if !st.Over || st.WinnerID == nil || *st.WinnerID != 3 {
		t.Errorf("over %v winner %v, want 3", st.Over, st.WinnerID)
	}
}

func TestRoundRobin(t *testing.T) {
	t.Parallel()
	for n := MinPlayers; n <= 6; n++ {
		var players []int64
		for i := range n {
			players = append(players, int64(i+1))
		}
		st := Plan(RoundRobin, players, nil)
		if len(st.Matches) != n*(n-1)/2 {
			t.Errorf("%d players: %d matches, want %d", n, len(st.Matches), n*(n-1)/2)
		}
		met := map[[2]int64]bool{}
		perRound := map[[2]int64]bool{} // player and round
		for _, m := range st.Matches {
			a, b := min(m.PlayerIDs[0], m.PlayerIDs[1]), max(m.PlayerIDs[0], m.PlayerIDs[1])
			if met[[2]int64{a, b}] || a == b || a == 0 {
				t.Errorf("%d players: bad or repeated match %v", n, m.PlayerIDs)
			}
			met[[2]int64{a, b}] = true
			for _, p := range m.PlayerIDs {
				if perRound[[2]int64{p, int64(m.Round)}] {
					t.Errorf("%d players: player %d twice in round %d", n, p, m.Round)
				}
				perRound[[2]int64{p, int64(m.Round)}] = true
			}
		}
	}
}

func TestStandings(t *testing.T) {
	t.Parallel()
	players := []int64{1, 2, 3}
	st := Plan(RoundRobin, players, nil)
	results := map[int]Result{}
	for _, m := range st.Matches {
		// 3 beats everyone; 1 beats 2 3:2, so 2 is last.
		a, b := m.PlayerIDs[0], m.PlayerIDs[1]
		r := Result{GameID: int64(m.Index + 1), Over: true}
		switch {
		case a == 3 || b == 3:
			r.WinnerID = 3
			r.Score = [2]int{3, 0}
			if b == 3 {
				r.Score = [2]int{0, 3}
			}
		default:
			r.WinnerID = 1
			r.Score = [2]int{3, 2}
			if b == 1 {
				r.Score = [2]int{2, 3}
			}
		}
		results[m.Index] = r
	}
	st = Plan(RoundRobin, players, results)
	var order []int64
	for _, s := range st.Standings {
		order = append(order, s.PlayerID)
	}
	if !slices.Equal(order, []int64{3, 1, 2}) || !st.Over || *st.WinnerID != 3 {
		t.Errorf("table %v over %v, want 3 1 2 with 3 winning", order, st.Over)
	}
	if s := st.Standings[1]; s.Played != 2 || s.Won != 1 || s.For != 3 || s.Against != 5 {
		t.Errorf("player 1: %+v, want 2 played, 1 won, 3:5", s)
	}
}

func TestKnockoutSizes(t *testing.T) {
	t.Parallel()
	for _, n := range []int{MinPlayers, 16, 17, MaxPlayers} {
		var players []int64
		for i := range n {
			players = append(players, int64(i+1))
		}
		st := Plan(Knockout, players, nil)
		size := 1
		for size < n {
			size *= 2
		}
		byes := 0
		for _, m := range st.Matches {
			if m.Bye {
				byes++
				if m.PlayerIDs[0] == 0 && m.PlayerIDs[1] == 0 {
					t.Errorf("%d players: match %d without players", n, m.Index)
				}
			}
		}
		if len(st.Matches) != size-1 || byes != size-n {
			t.Errorf("%d players: %d matches, %d byes, want %d and %d", n, len(st.Matches), byes, size-1, size-n)
		}
	}
}
