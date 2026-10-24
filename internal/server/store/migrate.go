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

// migrateSchema brings the database's schema up to date. Bun records the migrations that ran
// in the table bun_migrations.
func migrateSchema(ctx context.Context, db *bun.DB) error {
	dir := "migrations/sqlite"
	if db.Dialect().Name() == dialect.PG {
		dir = "migrations/postgres"
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
