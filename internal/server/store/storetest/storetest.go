// Package storetest makes PostgreSQL databases for the tests of the store and of the
// service.
package storetest

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"

	"github.com/uptrace/bun/driver/pgdriver"
)

// PostgresEnv names the variable with the data source name of a PostgreSQL database the
// tests may empty, e.g. postgres://postgres:test@localhost:5432/postgres?sslmode=disable
// for a server from
//
//	docker run --rm -e POSTGRES_PASSWORD=test -p 5432:5432 postgres
//
// Without it the tests that need PostgreSQL are skipped.
const PostgresEnv = "DARTS_O_MAT_TEST_POSTGRES"

// Postgres makes an empty database with the name on the server of PostgresEnv and returns
// its data source name. It skips the test if PostgresEnv isn't set.
func Postgres(t *testing.T, name string) string {
	t.Helper()
	dsn := os.Getenv(PostgresEnv)
	if dsn == "" {
		t.Skip(PostgresEnv + " isn't set")
	}
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
