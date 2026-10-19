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
	"slices"

	"github.com/sj14/darts-o-mat/common/throw"
)

const DartsPerTurn = 3

var (
	ErrGameOver = errors.New("game is already over")
	// ErrWaitingForTakeout means the turn is over, but the darts haven't been pulled out yet.
	ErrWaitingForTakeout = errors.New("pull out the darts first")
	// ErrWrongThrower means a dart is recorded for another player than the one who is up,
	// e.g. after a correction that changed whose turn it is.
	ErrWrongThrower = errors.New("the dart was thrown by another player than the one who is up")
	// ErrCantLeave means a player can't leave the game now: while bulling off, or when
	// that would leave fewer than two players.
	ErrCantLeave = errors.New("the player can't leave the game now")
)

// Log is what happened in a game, the input to replay it.
type Log struct {
	// Entries are the darts, takeouts (throw.Dart.Takeout), skipped turns
	// (throw.Dart.Skip) and players leaving (throw.Dart.Leave) in the order they happened.
	Entries []throw.Dart
	// PlayerIDs are the players in throwing order, to tell who a dart or a leaving
	// belongs to (throw.Dart.PlayerID); darts without a PlayerID aren't checked.
	PlayerIDs []int64
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
		Records: make([]Record, players), legDarts: make([]int, players), legs: max(l.Legs, 1), sets: max(l.Sets, 1), waitForTakeout: l.WaitForTakeout,
	}
	if l.BullOff && players > 1 {
		t.bull = newBullOff(players, l.BullOffCloser)
	}
	return t
}

// Replay calls dart for every dart of the log in order, except those thrown to bull
// off, and handles the takeouts in between. A dart while the turn waits for the
// takeout is an error. Afterwards t.Throwers tells who threw each entry, and t.Times
// when the game and its finished legs and sets were played.
func (l Log) Replay(t *Turn, dart func(i int, d throw.Dart) error) error {
	t.Throwers = make([]int, len(l.Entries))
	legFrom, setFrom := -1, -1 // the first dart of the current leg and set
	span := func(from, to int) Span {
		return Span{Start: l.Entries[from].CreatedAt, End: l.Entries[to].CreatedAt}
	}
	for i, d := range l.Entries {
		if d.IsDart() {
			if t.Times.Game.Start.IsZero() {
				t.Times.Game.Start = d.CreatedAt
			}
			t.Times.Game.End = d.CreatedAt
		}
		t.Throwers[i] = t.Player
		switch {
		case d.Leave:
			t.Throwers[i] = -1
			if err := t.Leave(slices.Index(l.PlayerIDs, d.PlayerID)); err != nil {
				return fmt.Errorf("entry %d: %w", i+1, err)
			}
			continue
		case d.Skip:
			t.Throwers[i] = -1
			if len(l.PlayerIDs) > 0 && d.PlayerID != l.PlayerIDs[t.Player] {
				return fmt.Errorf("entry %d: %w", i+1, ErrWrongThrower)
			}
			t.Skip()
			continue
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
		case len(l.PlayerIDs) > 0 && d.PlayerID != 0 && d.PlayerID != l.PlayerIDs[t.Player]:
			return fmt.Errorf("dart %d: %w", i+1, ErrWrongThrower)
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
			t.Times.Legs = append(t.Times.Legs, LegTime{Set: set, Leg: leg, Span: span(legFrom, i)})
			legFrom = -1
			if t.setWinner != nil || t.over {
				t.Times.Sets = append(t.Times.Sets, span(setFrom, i))
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
	Player int // index of the player whose turn it is; the winner once the game is over
	Round  int // of the current leg, starts at 1
	Dart   int // darts already thrown in this turn
	Set    int // starts at 1; always 1 without sets
	Leg    int // in the current set, starts at 1
	// Records are what each player has won so far; the modes embed them in their players.
	Records []Record
	// OnLegStart is called when the next leg starts, so the game mode resets the scores.
	OnLegStart func()
	// Throwers is the index of the player who threw each log entry, -1 for the others.
	Throwers []int
	// Times is when the game and its finished legs and sets were played (see Replay).
	Times Times

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
	r := &t.Records[t.Player]
	if r.BestLeg == 0 || t.legDarts[t.Player] < r.BestLeg {
		r.BestLeg = t.legDarts[t.Player]
	}
	r.Legs++
	r.SetLegs++
	winner := t.Player
	t.legWinner = &winner
	if r.SetLegs < t.legs {
		t.pass(nextLeg)
		return
	}
	r.Sets++
	if r.Sets >= t.sets {
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
		from := t.Player
		t.Player = t.nextActive(from)
		if t.reaches(from, t.Player, t.starter) {
			t.Round++
		}
	case nextLeg:
		t.Leg++
		t.starter = t.nextActive(t.starter)
		t.startLeg()
	case nextSet:
		t.Set, t.Leg = t.Set+1, 1
		for i := range t.Records {
			t.Records[i].SetLegs = 0
		}
		t.setStarter = t.nextActive(t.setStarter)
		t.starter = t.setStarter
		t.startLeg()
	}
}

// reaches reports whether moving from player from to player to arrives at or passes
// player at, going round: the round is over when the starter is reached. A starter who
// left is passed all the same.
func (t *Turn) reaches(from, to, at int) bool {
	n := len(t.Records)
	for k := (from + 1) % n; ; k = (k + 1) % n {
		if k == at {
			return true
		}
		if k == to {
			return false
		}
	}
}

// nextActive returns the player who throws after player i, skipping those who left.
func (t *Turn) nextActive(i int) int {
	n := len(t.Records)
	for j := 1; j < n; j++ {
		if k := (i + j) % n; !t.Records[k].Left {
			return k
		}
	}
	return i
}

// Leave lets player i leave the game: the others play on without them, and the legs and
// sets they won stay theirs; a starter who left still counts for the order in which the
// legs and sets are started (the next one after them starts). If they were up, their turn is over (the darts they threw
// count); while the turn waits for the takeout, the next step moves on from them. It is
// refused while bulling off, after the game is over, and when fewer than two players
// would be left (the game ends then, which is up to the caller).
func (t *Turn) Leave(i int) error {
	active := 0
	for _, r := range t.Records {
		if !r.Left {
			active++
		}
	}
	switch {
	case i < 0 || i >= len(t.Records) || t.Records[i].Left:
		return fmt.Errorf("%w: not a player of the game", ErrCantLeave)
	case t.over:
		return ErrGameOver
	case t.BullingOff():
		return fmt.Errorf("%w: the players are bulling off", ErrCantLeave)
	case active <= 2:
		return fmt.Errorf("%w: it would leave fewer than two players", ErrCantLeave)
	}
	t.Records[i].Left = true
	if t.Player == i && !t.Waiting() {
		t.do(nextPlayer)
	}
	return nil
}

// Skip ends the turn of the player who is up, who took too long: the darts they threw
// count and the next player is up. Nothing happens while bulling off, while the turn
// waits for the takeout (the darts are pulled out whenever the players like) and after
// the game is over.
func (t *Turn) Skip() {
	if !t.over && !t.BullingOff() && !t.Waiting() {
		t.do(nextPlayer)
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
