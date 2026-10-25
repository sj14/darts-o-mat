package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun/driver/pgdriver"
)

// The unique index keeps an email to one account even when the check before it is skipped,
// as when two requests race, and lets any number of accounts have none.
func TestEmailIndex(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	insert := func(username, email string) error {
		a := account.Account{Username: username, Email: email, PasswordHash: "x", PlayerID: int64(len(username))}
		_, err := st.db.NewInsert().Model(&a).Exec(ctx)
		return err
	}
	if err := insert("aaa", ""); err != nil {
		t.Fatal(err)
	}
	if err := insert("bbbb", ""); err != nil {
		t.Errorf("a second account without an email: %v", err)
	}
	if err := insert("ccccc", "c@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := insert("dddddd", "c@example.com"); err == nil {
		t.Error("two accounts with the same email were stored")
	}
}

// The store keeps the last admin by itself, also when the service's check before it is
// skipped: neither taking the role nor deleting the account goes through.
func TestKeepLastAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	anna, ben := createAccount(t, st, "anna"), createAccount(t, st, "ben") // anna is the admin

	if err := st.SetAdmin(ctx, anna.ID, false); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("taking the role from the last admin: %v, want ErrLastAdmin", err)
	}
	if err := st.DeleteAccount(ctx, anna); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("deleting the last admin: %v, want ErrLastAdmin", err)
	}
	if err := st.SetAdmin(ctx, ben.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetAdmin(ctx, anna.ID, false); err != nil {
		t.Errorf("taking the role from one of two admins: %v", err)
	}
	if err := st.DeleteAccount(ctx, ben); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("deleting ben, now the last admin: %v, want ErrLastAdmin", err)
	}
	if err := st.DeleteAccount(ctx, anna); err != nil {
		t.Errorf("deleting an account that isn't an admin: %v", err)
	}
}

// On PostgreSQL two transactions that each take the role from one of the last two admins
// don't both go through: the second waits for the first one's lock and then sees that its
// admin is the only one left. Needs DARTS_O_MAT_TEST_POSTGRES like
// TestPostgresMigrationsMatchModels, but uses a database of its own, since that test
// empties the schema.
func TestKeepLastAdminPostgres(t *testing.T) {
	t.Parallel()
	dsn := os.Getenv("DARTS_O_MAT_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("DARTS_O_MAT_TEST_POSTGRES isn't set")
	}
	ctx := context.Background()
	st := newPostgresDatabase(t, dsn, "darts_o_mat_keep_last_admin")
	anna, ben := createAccount(t, st, "anna"), createAccount(t, st, "ben")
	if err := st.SetAdmin(ctx, ben.ID, true); err != nil {
		t.Fatal(err)
	}

	// The first transaction takes the role from anna and holds on.
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := keepAdmin(ctx, tx, anna.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.NewUpdate().Model((*account.Account)(nil)).Set("admin = false").Where("id = ?", anna.ID).Exec(ctx); err != nil {
		t.Fatal(err)
	}

	// The second takes it from ben meanwhile: it has to wait.
	done := make(chan error, 1)
	go func() { done <- st.SetAdmin(ctx, ben.ID, false) }()
	select {
	case err := <-done:
		t.Fatalf("taking the role from ben didn't wait for the first transaction: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, ErrLastAdmin) {
		t.Errorf("taking the role from ben after anna's: %v, want ErrLastAdmin", err)
	}
}

// createAccount stores an account with its player; the first one is the admin.
func createAccount(t *testing.T, st *Store, username string) account.Account {
	t.Helper()
	a := account.Account{Username: username, PasswordHash: "x"}
	if err := st.CreateAccount(context.Background(), &a, &player.Player{Name: username}); err != nil {
		t.Fatal(err)
	}
	return a
}

// newPostgresDatabase makes an empty database with the name on the PostgreSQL server of
// dsn and opens the store on it.
func newPostgresDatabase(t *testing.T, dsn, name string) *Store {
	t.Helper()
	return openPostgres(t, emptyPostgresDatabase(t, dsn, name))
}

// emptyPostgresDatabase makes an empty database with the name on the PostgreSQL server of
// dsn and returns its dsn.
func emptyPostgresDatabase(t *testing.T, dsn, name string) string {
	t.Helper()
	ctx := context.Background()
	server := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	defer server.Close()
	for _, query := range []string{"DROP DATABASE IF EXISTS " + name + " WITH (FORCE)", "CREATE DATABASE " + name} {
		if _, err := server.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}
