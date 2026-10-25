package store

import (
	"context"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
)

// The lock is held by one replica at a time; one that gives up waiting doesn't get it.
// Needs DARTS_O_MAT_TEST_POSTGRES like TestPostgresMigrationsMatchModels.
func TestLockPostgres(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := storetest.Postgres(t, "darts_o_mat_lock")
	one, two := openPostgres(t, dsn), openPostgres(t, dsn)

	unlock, err := one.Lock(ctx)
	if err != nil {
		t.Fatal(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, err := two.Lock(waitCtx); err == nil {
		t.Fatal("two replicas hold the lock")
	}

	got := make(chan func(), 1)
	go func() {
		unlock, err := two.Lock(ctx)
		if err != nil {
			t.Error(err)
		}
		got <- unlock
	}()
	select {
	case <-got:
		t.Fatal("the lock was taken while held")
	case <-time.After(200 * time.Millisecond):
	}
	unlock()
	select {
	case unlock := <-got:
		unlock()
	case <-time.After(10 * time.Second):
		t.Fatal("the lock wasn't taken once it was free")
	}
}
