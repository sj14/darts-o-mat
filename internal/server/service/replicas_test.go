package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/store"
	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
	"golang.org/x/crypto/bcrypt"
)

// newReplicas opens n services on one PostgreSQL database, each with its own connections
// and bus, like replicas of the server; they listen until the test ends.
func newReplicas(t *testing.T, name string, n int) []*Service {
	t.Helper()
	dsn := storetest.Postgres(t, name)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	var replicas []*Service
	for range n {
		st, err := store.NewPostgres(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		s := New(st, Options{PasswordCost: bcrypt.MinCost, Bus: st.Bus()})
		s.shuffle = func(int, func(i, j int)) {}
		go s.Listen(ctx)
		replicas = append(replicas, s)
	}
	return replicas
}

// A dart through one replica reaches the pages streaming from another, and darts through
// several replicas at once are ordered by the lock: each is played on the state the one
// before left. Needs storetest.PostgresEnv.
func TestReplicasPostgres(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	replicas := newReplicas(t, "darts_o_mat_replicas", 2)
	one, two := replicas[0], replicas[1]

	anna := register(t, one, "anna")
	guest, err := one.CreateGuest(ctx, anna, "Gast")
	if err != nil {
		t.Fatal(err)
	}
	v, err := createGame(ctx, one, anna, x01Game(anna.PlayerID, guest.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID

	changes, stop := two.Subscribe(id)
	defer stop()
	wait := func(what string) {
		t.Helper()
		select {
		case <-changes:
		case <-time.After(10 * time.Second):
			t.Fatalf("the second replica didn't hear %s", what)
		}
	}
	// The second replica may not listen yet: notify until it hears.
	for listening := false; !listening; {
		one.notify(id)
		select {
		case <-changes:
			listening = true
		case <-time.After(100 * time.Millisecond):
		}
	}
	time.Sleep(100 * time.Millisecond) // notifications still on their way
	select {
	case <-changes:
	default:
	}
	if _, err := one.AddDart(ctx, anna, id, t20); err != nil {
		t.Fatal(err)
	}
	wait("the dart")

	// Two visits of three darts, the darts thrown through both replicas at once.
	const darts = 5
	var wg sync.WaitGroup
	for i := range darts {
		s := replicas[i%2]
		wg.Go(func() {
			if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
				t.Errorf("a dart: %v", err)
			}
		})
	}
	wg.Wait()
	v, err = two.Game(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(v.Darts); got != 1+darts {
		t.Errorf("%d darts in the log, want %d", got, 1+darts)
	}
	for _, p := range v.Players {
		if got := v.State.Common().Records[p.ID]; got.Legs != 0 {
			t.Errorf("%s won a leg", p.Name)
		}
	}
	if got := v.State.Common().Round; got != 2 {
		t.Errorf("round %d, want 2 after both played a visit", got)
	}
}
