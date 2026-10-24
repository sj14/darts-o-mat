package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/migrate"
)

// migrationFiles are the schema's changes, one directory per database with the same names:
// since-<tag>/<time>_<what>.tx.up.sql, grouped by the release they came after, each run in a
// transaction, statements split by "--bun:split". Bun finds them in the subdirectories too.
//
//go:embed migrations
var migrationFiles embed.FS

// migrationLock is the key of the PostgreSQL advisory lock around the migrations; any
// number that nothing else on the database locks.
const migrationLock = 0x6461727473 // "darts"

// migrateSchema brings the database's schema up to date. Bun records the migrations that ran
// in the table bun_migrations.
//
// On PostgreSQL it holds an advisory lock meanwhile, so replicas starting together don't
// run the same migration twice: the second waits and then finds nothing to do. The lock
// belongs to the connection, so it is gone when the process dies. SQLite is one file in
// one process.
func migrateSchema(ctx context.Context, db *bun.DB) error {
	dir := "migrations/sqlite"
	if db.Dialect().Name() == dialect.PG {
		dir = "migrations/postgres"
		conn, err := db.Conn(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(?)", migrationLock); err != nil {
			return fmt.Errorf("lock the migrations: %w", err)
		}
		// Closing only returns the connection to the pool, which keeps the lock.
		defer conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock(?)", migrationLock)
	}
	files, err := fs.Sub(migrationFiles, dir)
	if err != nil {
		return err
	}
	migrations := migrate.NewMigrations()
	if err := migrations.Discover(files); err != nil {
		return err
	}
	migrator := migrate.NewMigrator(db, migrations, migrate.WithMarkAppliedOnSuccess(true))
	if err := migrator.Init(ctx); err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("migrate the database: %w", err)
	}
	return nil
}
