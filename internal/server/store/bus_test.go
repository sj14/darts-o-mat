package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
)

// A notification published through one replica reaches the listener of another, and the
// listener says once at the start that it may have missed some. Needs
// DARTS_O_MAT_TEST_POSTGRES like TestPostgresMigrationsMatchModels.
func TestBusPostgres(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsn := storetest.Postgres(t, "darts_o_mat_bus")
	// Two stores on the same database, like two replicas.
	sender, receiver := openPostgres(t, dsn), openPostgres(t, dsn)

	received := make(chan []string, 1)
	missed := make(chan struct{}, 1)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		receiver.Bus().Listen(ctx, func(keys []string) { received <- keys }, func() { missed <- struct{}{} })
	}()
	select {
	case <-missed:
	case <-time.After(10 * time.Second):
		t.Fatal("the listener didn't start")
	}

	if err := sender.Bus().Publish(ctx, "g1", "t2"); err != nil {
		t.Fatal(err)
	}
	select {
	case keys := <-received:
		if want := []string{"g1", "t2"}; !slices.Equal(keys, want) {
			t.Errorf("received %q, want %q", keys, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nothing received")
	}

	// After its connection is gone the listener connects again and says it may have missed
	// notifications.
	if _, err := sender.db.ExecContext(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = current_database() AND query LIKE 'LISTEN%'"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-missed:
	case <-time.After(10 * time.Second):
		t.Fatal("the listener didn't connect again")
	}
	if err := sender.Bus().Publish(ctx, "g3"); err != nil {
		t.Fatal(err)
	}
	select {
	case keys := <-received:
		if want := []string{"g3"}; !slices.Equal(keys, want) {
			t.Errorf("received %q after connecting again, want %q", keys, want)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("nothing received after connecting again")
	}

	cancel()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("the listener didn't stop with its context")
	}
}

func openPostgres(t *testing.T, dsn string) *Store {
	t.Helper()
	st, err := NewPostgres(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}
