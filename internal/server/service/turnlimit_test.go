package service

import (
	"context"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
)

func TestTurnLimit(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, carla := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "carla")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, carla)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }

	g := x01Game(anna.PlayerID)
	g.TurnSeconds = 60
	v, err := s.CreateGame(ctx, anna, g, []string{"ben", "carla"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	if v, _ = s.Game(ctx, anna, id); v.Game.StartedAt != nil || s.timers[id] != nil {
		t.Fatalf("started with carla's invitation open: %v, timer %v", v.Game.StartedAt, s.timers[id])
	}
	_, _ = s.AcceptInvitation(ctx, carla, id, nil)

	// The clock starts with the game, before the first dart.
	if v, _ = s.Game(ctx, anna, id); v.Game.StartedAt == nil || s.timers[id] == nil {
		t.Fatalf("after everyone accepted: started %v, timer %v; want both", v.Game.StartedAt, s.timers[id])
	}
	s.endTurn(id) // still in time
	if v, _ = s.Game(ctx, anna, id); len(v.Darts) != 0 {
		t.Fatalf("a turn in time was ended: %d entries", len(v.Darts))
	}

	// Each turn that runs out is skipped; anna's third in a row makes her leave.
	// The skipped turns: anna, ben, carla, anna (2), ben, carla, anna (3).
	for i, want := range []int64{anna.PlayerID, ben.PlayerID, carla.PlayerID, anna.PlayerID, ben.PlayerID, carla.PlayerID} {
		offset += 2 * time.Minute
		s.endTurn(id)
		v, _ = s.Game(ctx, anna, id)
		last := v.Darts[len(v.Darts)-1]
		if !last.Skip || last.PlayerID != want {
			t.Fatalf("turn %d: last entry %+v, want a skip of player %d", i+1, last, want)
		}
	}
	offset += 2 * time.Minute
	s.endTurn(id)
	v, _ = s.Game(ctx, ben, id)
	if last := v.Darts[len(v.Darts)-1]; !last.Leave || last.PlayerID != anna.PlayerID || !v.Common().Records[anna.PlayerID].Left {
		t.Fatalf("after the third skip: last entry %+v, want anna leaving", last)
	}
	if cur := v.Common().CurrentPlayerID; cur == nil || *cur != ben.PlayerID {
		t.Fatalf("current player %v, want ben", cur)
	}

	// A dart starts the count again; with two players left the next one to run out ends the game.
	if _, err := s.AddDart(ctx, ben, id, t20); err != nil {
		t.Fatal(err)
	}
	for range 2 { // ben's rest of the turn, carla
		offset += 2 * time.Minute
		s.endTurn(id)
	}
	for range 3 {
		offset += 2 * time.Minute
		s.endTurn(id)
	}
	if v, _ = s.Game(ctx, ben, id); !v.Game.Aborted() {
		t.Errorf("game after carla ran out three times with ben: aborted %v, entries %d; want it aborted", v.Game.Aborted(), len(v.Darts))
	}
	if s.timers[id] != nil {
		t.Error("an aborted game still has a timer")
	}
}

// The time the server was down doesn't cost a turn: after a start every turn has its whole
// time again.
func TestTurnLimitAfterRestart(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	g := x01Game(anna.PlayerID)
	g.TurnSeconds = 60
	v, err := s.CreateGame(ctx, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID

	// The server was down for five minutes and starts again.
	offset = 5 * time.Minute
	s.started = s.now()
	if v, _ = s.Game(ctx, anna, id); v.TurnDeadline == nil || !v.TurnDeadline.Equal(s.started.Add(time.Minute)) {
		t.Fatalf("deadline after the start %v, want a minute after it (%v)", v.TurnDeadline, s.started.Add(time.Minute))
	}
	s.endTurn(id)
	if v, _ = s.Game(ctx, anna, id); len(v.Darts) != 0 {
		t.Fatalf("the turn ended right after the start: %d entries", len(v.Darts))
	}
	offset += 2 * time.Minute
	s.endTurn(id)
	if v, _ = s.Game(ctx, anna, id); len(v.Darts) != 1 || !v.Darts[0].Skip {
		t.Errorf("a minute after the start: %+v, want the turn skipped", v.Darts)
	}
}

func TestSkippedTurnsCantBeTakenBack(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }
	g := x01Game(anna.PlayerID)
	g.TurnSeconds = 60
	v, _ := s.CreateGame(ctx, anna, g, []string{"ben"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AddDart(ctx, anna, id, t20)
	offset += 2 * time.Minute
	s.endTurn(id)
	if v, _ = s.Game(ctx, anna, id); v.MayUndo() || !v.Darts[1].Skip || v.Darts[1].IsDart() {
		t.Errorf("entry after the skipped turn: %+v, may undo %v; want a skip that is no dart and can't be undone", v.Darts[1], v.MayUndo())
	}
	// A client can't enter a skip as a dart.
	d := throw.Dart{Number: 5, Segment: dartboard.Single, Skip: true}
	if v, err := s.AddDart(ctx, ben, id, d); err != nil || !v.Darts[len(v.Darts)-1].IsDart() {
		t.Errorf("dart with the skip flag set: %v, want a normal dart", err)
	}
}

func TestResumeTurnLimits(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	g := x01Game(anna.PlayerID)
	g.TurnSeconds = 60
	v, _ := s.CreateGame(ctx, anna, g, []string{"ben"})
	id := v.Game.ID
	_, _ = s.AcceptInvitation(ctx, ben, id, nil)
	_, _ = s.AddDart(ctx, anna, id, t20)

	s.timersMu.Lock()
	clear(s.timers) // the server was restarted
	s.timersMu.Unlock()
	if err := s.ResumeTurnLimits(ctx); err != nil {
		t.Fatal(err)
	}
	if s.timers[id] == nil {
		t.Error("no timer after resuming")
	}
}

func TestTurnLimitValidation(t *testing.T) {
	for seconds, ok := range map[int]bool{0: true, game.MinTurnSeconds: true, game.MaxTurnSeconds: true, -1: false, 5: false, game.MaxTurnSeconds + 1: false} {
		s := x01Game(1).Settings
		s.Legs, s.Sets, s.TurnSeconds = 1, 1, seconds
		if err := s.Validate(); (err == nil) != ok {
			t.Errorf("%d seconds per turn: got %v, want valid %v", seconds, err, ok)
		}
	}
}

// A game starts when it is ready, however that came about.
func TestGameStart(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, cara, ben)
	// A player plays one game at a time: each game is of other players.
	guest, _ := s.CreateGuest(ctx, anna, "Gast")

	alone, _ := s.CreateGame(ctx, anna, x01Game(guest.ID), nil)
	if alone.Game.StartedAt == nil {
		t.Error("a game of one account isn't started when created")
	}
	open := x01Game(anna.PlayerID)
	open.Seats = 2
	v, _ := s.CreateGame(ctx, anna, open, nil)
	if v.Game.StartedAt != nil {
		t.Fatal("an open game started with a free seat")
	}
	if v, _ = s.JoinGame(ctx, ben, v.Game.ID, nil); v.Game.StartedAt == nil {
		t.Error("an open game didn't start with its last seat")
	}
	// The last invitation open is declined: the others start.
	v, _ = s.CreateGame(ctx, cara, x01Game(cara.PlayerID), []string{"ben"})
	if v.Game.StartedAt != nil {
		t.Fatal("started with an invitation open")
	}
	if err := s.DeclineInvitation(ctx, ben, v.Game.ID); err != nil {
		t.Fatal(err)
	}
	if v, _ = s.Game(ctx, cara, v.Game.ID); v.Game.StartedAt == nil {
		t.Error("the game didn't start after the invitation was declined")
	}
}
