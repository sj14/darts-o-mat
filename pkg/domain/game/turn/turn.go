// Package turn contains the order of play shared by all game modes: the players
// throw three darts each, one after another.
//
// A game is played over legs. The player who finishes wins the leg at once; whoever
// first wins the agreed number of legs wins the game. Each leg is started by the next
// player in turn.
//
// Optionally the legs are grouped into sets: whoever first wins the agreed number of
// legs wins the set, whoever first wins the agreed number of sets the game. The first
// leg of each set is started by the next player in turn after the one who started the
// previous set; within the set the legs go round from there.
//
// Optionally the players bull off first: each throws one dart at the bull, the closest
// starts the first leg. Players who are equally close throw again.
//
// Games played on a board with cameras wait after each turn until the darts were
// pulled out (a takeout), so the next player doesn't start while the darts are
// still in the board.
package turn

import (
	"errors"
	"fmt"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

const DartsPerTurn = 3

var (
	ErrGameOver = errors.New("game is already over")
	// ErrWaitingForTakeout means the turn is over, but the darts haven't been pulled out yet.
	ErrWaitingForTakeout = errors.New("pull out the darts first")
)

// Log is what happened in a game, the input to replay it.
type Log struct {
	// Entries are the darts and takeouts (throw.Dart.Takeout) in the order they happened.
	Entries []throw.Dart
	// WaitForTakeout keeps a finished turn until the darts were pulled out.
	WaitForTakeout bool
	// Legs is how many legs win the game, or a set when playing sets; 0 counts as 1.
	Legs int
	// Sets is how many sets win the game; 0 or 1 is a game without sets.
	Sets int
	// BullOff lets the players bull off for who starts (with more than one player).
	BullOff bool
	// BullOffCloser is the index of the player the players found closer when bulling
	// off ended in a tie (e.g. both hit the bull, entered by hand), nil if they didn't
	// decide. It takes the place of throwing again.
	BullOffCloser *int
}

// Turn returns the order of play for the log's settings.
func (l Log) Turn(players int) *Turn {
	t := &Turn{
		Round: 1, Set: 1, Leg: 1,
		LegsWon: make([]int, players), SetLegs: make([]int, players), SetsWon: make([]int, players), BestLeg: make([]int, players),
		legDarts: make([]int, players), legs: max(l.Legs, 1), sets: max(l.Sets, 1), waitForTakeout: l.WaitForTakeout,
	}
	if l.BullOff && players > 1 {
		t.bull = newBullOff(players, l.BullOffCloser)
	}
	return t
}

// Replay calls dart for every dart of the log in order, except those thrown to bull
// off, and handles the takeouts in between. A dart while the turn waits for the
// takeout is an error. Afterwards t.Throwers tells who threw each entry, and t.Time,
// t.Legs and t.Sets when the game and its finished legs and sets were played.
func (l Log) Replay(t *Turn, dart func(i int, d throw.Dart) error) error {
	t.Throwers = make([]int, len(l.Entries))
	legFrom, setFrom := -1, -1 // the first dart of the current leg and set
	span := func(from, to int) Span {
		return Span{Start: l.Entries[from].CreatedAt, End: l.Entries[to].CreatedAt}
	}
	for i, d := range l.Entries {
		if !d.Takeout {
			if t.Time.Start.IsZero() {
				t.Time.Start = d.CreatedAt
			}
			t.Time.End = d.CreatedAt
		}
		t.Throwers[i] = t.Player
		switch {
		case d.Takeout:
			// Bull-off darts are pulled out whenever the players like.
			t.Throwers[i] = -1
			if !t.BullingOff() {
				t.Takeout()
			}
			continue
		case t.Waiting():
			return fmt.Errorf("entry %d: %w", i+1, ErrWaitingForTakeout)
		case t.Over():
			return fmt.Errorf("dart %d: %w", i+1, ErrGameOver)
		case t.BullingOff():
			if err := d.Validate(); err != nil {
				return fmt.Errorf("dart %d: %w", i+1, err)
			}
			next, decided := t.bull.throw(i, t.Player, d)
			t.Player = next
			if decided {
				t.starter, t.setStarter = next, next
			}
			continue
		}
		t.legWinner, t.setWinner = nil, nil
		set, leg := t.Set, t.Leg // the dart may start the next leg
		if legFrom < 0 {
			legFrom = i
		}
		if setFrom < 0 {
			setFrom = i
		}
		if err := dart(i, d); err != nil {
			return err
		}
		if t.legWinner != nil || t.over {
			t.Legs = append(t.Legs, LegTime{Set: set, Leg: leg, Span: span(legFrom, i)})
			legFrom = -1
			if t.setWinner != nil || t.over {
				t.Sets = append(t.Sets, span(setFrom, i))
				setFrom = -1
			}
		}
	}
	return nil
}

// afterTakeout is what happens when the darts of a finished turn are pulled out.
type afterTakeout uint8

const (
	nothing    afterTakeout = iota // not waiting
	nextPlayer                     // the next player throws
	nextLeg                        // the turn won a leg, the next leg starts
	nextSet                        // the turn won a set, the next set starts
)

type Turn struct {
	Player  int   // index of the player whose turn it is; the winner once the game is over
	Round   int   // of the current leg, starts at 1
	Dart    int   // darts already thrown in this turn
	Set     int   // starts at 1; always 1 without sets
	Leg     int   // in the current set, starts at 1
	LegsWon []int // per player, over the whole game
	SetLegs []int // per player: legs won in the current set (all legs without sets)
	SetsWon []int // per player
	BestLeg []int // per player: fewest darts in a won leg, 0 without one
	// OnLegStart is called when the next leg starts, so the game mode resets the scores.
	OnLegStart func()
	// Throwers is the index of the player who threw each log entry, -1 for takeouts.
	Throwers []int
	// Time is when the game was played so far, from its first dart (bulling off
	// included) to its last; Legs and Sets when each finished leg and set was played.
	Time Span
	Legs []LegTime
	Sets []Span

	legs           int   // to win a set (the game without sets)
	sets           int   // to win the game
	legDarts       []int // per player, in the current leg
	starter        int   // the player who began the current leg
	setStarter     int   // the player who began the current set
	waitForTakeout bool
	waiting        afterTakeout
	over           bool
	legWinner      *int     // who won a leg with the last dart, while the game goes on
	setWinner      *int     // who won a set with the last dart, while the game goes on
	bull           *bullOff // nil without bulling off
}

// Advance counts a dart thrown; the turn is over after the third dart.
func (t *Turn) Advance() {
	t.count()
	if t.Dart == DartsPerTurn {
		t.pass(nextPlayer)
	}
}

// End counts a dart that ends the turn early, e.g. a bust.
func (t *Turn) End() {
	t.count()
	t.pass(nextPlayer)
}

// Finish counts the dart the current player finished with: they win the leg, the set
// once they have won enough legs, and the game once they have won enough sets.
func (t *Turn) Finish() {
	t.count()
	if best := t.BestLeg[t.Player]; best == 0 || t.legDarts[t.Player] < best {
		t.BestLeg[t.Player] = t.legDarts[t.Player]
	}
	t.LegsWon[t.Player]++
	t.SetLegs[t.Player]++
	winner := t.Player
	t.legWinner = &winner
	if t.SetLegs[t.Player] < t.legs {
		t.pass(nextLeg)
		return
	}
	t.SetsWon[t.Player]++
	if t.SetsWon[t.Player] >= t.sets {
		t.legWinner, t.over = nil, true
		return
	}
	t.setWinner = &winner
	t.pass(nextSet)
}

// count counts a dart of the current player.
func (t *Turn) count() {
	t.Dart++
	t.legDarts[t.Player]++
}

// pass ends the turn: then comes the next player or leg, or, when waiting for takeouts,
// after the darts were pulled out.
func (t *Turn) pass(next afterTakeout) {
	if t.waitForTakeout {
		t.waiting = next
		return
	}
	t.do(next)
}

func (t *Turn) do(next afterTakeout) {
	switch next {
	case nextPlayer:
		t.Dart = 0
		t.Player = (t.Player + 1) % len(t.LegsWon)
		if t.Player == t.starter {
			t.Round++
		}
	case nextLeg:
		t.Leg++
		t.starter = (t.starter + 1) % len(t.LegsWon)
		t.startLeg()
	case nextSet:
		t.Set, t.Leg = t.Set+1, 1
		clear(t.SetLegs)
		t.setStarter = (t.setStarter + 1) % len(t.LegsWon)
		t.starter = t.setStarter
		t.startLeg()
	}
}

// startLeg starts a leg with t.starter.
func (t *Turn) startLeg() {
	t.Round, t.Dart = 1, 0
	clear(t.legDarts)
	t.Player = t.starter
	if t.OnLegStart != nil {
		t.OnLegStart()
	}
}

// Takeout handles the darts being pulled out: a turn waiting for it, or one with fewer
// than three darts (the others bounced off), passes on.
func (t *Turn) Takeout() {
	switch {
	case t.waiting != nothing:
		next := t.waiting
		t.waiting = nothing
		t.do(next)
	case t.Dart > 0 && !t.over:
		t.do(nextPlayer)
	}
}

// Waiting reports whether the turn is over but the darts weren't pulled out yet.
func (t *Turn) Waiting() bool {
	return t.waiting != nothing
}

func (t *Turn) Over() bool {
	return t.over
}
