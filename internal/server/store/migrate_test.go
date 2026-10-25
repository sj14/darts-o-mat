package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/site"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/store/storetest"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/migrate"
)

// models are the structs with a table; the migrations have to give each its table.
var models = []any{
	(*account.Account)(nil),
	(*account.Session)(nil),
	(*account.Friendship)(nil),
	(*game.Game)(nil),
	(*x01Row)(nil),
	(*aroundTheClockRow)(nil),
	(*game.Participant)(nil),
	(*player.Player)(nil),
	(*board.Board)(nil),
	(*board.Token)(nil),
	(*throw.Dart)(nil),
	(*stats.Result)(nil),
	(*stats.TournamentResult)(nil),
	(*stats.Quit)(nil),
	(*player.Rating)(nil),
	(*tournament.Tournament)(nil),
	(*tournament.Participant)(nil),
	(*page.Page)(nil),
	(*site.Settings)(nil),
	(*serverStart)(nil),
}

// The SQLite migrations, written by hand, give the same schema as the structs: tables,
// columns with their types, defaults and keys, and indexes.
func TestSQLiteMigrationsMatchModels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	open := func(name string) *bun.DB {
		sqldb, err := sql.Open("sqlite", "file:"+t.Name()+name+"?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}
		sqldb.SetMaxOpenConns(1)
		t.Cleanup(func() { sqldb.Close() })
		return bun.NewDB(sqldb, sqlitedialect.New())
	}

	migrated := open("migrated")
	if err := migrateSchema(ctx, migrated); err != nil {
		t.Fatal(err)
	}
	// Running them again does nothing.
	if err := migrateSchema(ctx, migrated); err != nil {
		t.Fatal(err)
	}

	fromModels := open("models")
	for _, m := range models {
		if _, err := fromModels.NewCreateTable().Model(m).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := fromModels.NewCreateIndex().Model((*account.Account)(nil)).Index("accounts_email_key").
		Column("email").Unique().Where("email <> ''").Exec(ctx); err != nil {
		t.Fatal(err)
	}

	if got, want := sqliteSchema(t, migrated), sqliteSchema(t, fromModels); got != want {
		t.Errorf("the migrations give\n%s\nthe structs\n%s", got, want)
	}
}

// sqliteSchema describes the tables and indexes of the database, apart from Bun's own.
func sqliteSchema(t *testing.T, db *bun.DB) string {
	t.Helper()
	ctx := context.Background()
	var tables []string
	if err := db.NewRaw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'bun_%' AND name NOT LIKE 'sqlite_%' ORDER BY name").Scan(ctx, &tables); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, table := range tables {
		fmt.Fprintf(&b, "%s\n", table)
		rows, err := db.QueryContext(ctx, "SELECT name, type, \"notnull\", coalesce(dflt_value, ''), pk FROM pragma_table_info(?) ORDER BY name", table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var name, typ, dflt string
			var notnull, pk int
			if err := rows.Scan(&name, &typ, &notnull, &dflt, &pk); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&b, "  %s %s notnull=%d default=%s pk=%d\n", name, typ, notnull, dflt, pk)
		}
		rows.Close()

		var indexes []string
		rows, err = db.QueryContext(ctx, "SELECT il.\"unique\", il.partial, group_concat(ii.name) FROM pragma_index_list(?) il, pragma_index_info(il.name) ii GROUP BY il.name", table)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var unique, partial int
			var columns string
			if err := rows.Scan(&unique, &partial, &columns); err != nil {
				t.Fatal(err)
			}
			indexes = append(indexes, fmt.Sprintf("  index (%s) unique=%d partial=%d\n", columns, unique, partial))
		}
		rows.Close()
		slices.Sort(indexes)
		b.WriteString(strings.Join(indexes, ""))
	}
	return b.String()
}

// The PostgreSQL migrations give the schema of the structs: Bun's AutoMigrator finds nothing
// to change. Needs a PostgreSQL database that may be emptied (storetest.PostgresEnv).
//
// When it fails, it prints the SQL that is missing: the start of a new migration.
func TestPostgresMigrationsMatchModels(t *testing.T) {
	t.Parallel()
	dsn := os.Getenv(storetest.PostgresEnv)
	if dsn == "" {
		t.Skip(storetest.PostgresEnv + " isn't set")
	}
	ctx := context.Background()
	db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
	defer db.Close()
	if _, err := db.ExecContext(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}

	if err := migrateSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := migrateSchema(ctx, db); err != nil {
		t.Fatal(err)
	}

	auto, err := migrate.NewAutoMigrator(db, migrate.WithModel(models...), migrate.WithMigrationsDirectoryAuto(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	files, err := auto.CreateTxSQLMigrations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name, ".up.sql") {
			t.Errorf("the migrations miss:\n%s", f.Content)
		}
	}
}

// Replicas that start together don't run a migration twice: on PostgreSQL the advisory
// lock lets one migrate while the others wait, and each ran migration is recorded once.
// Needs DARTS_O_MAT_TEST_POSTGRES like TestPostgresMigrationsMatchModels, but uses a
// database of its own.
func TestPostgresMigrationsConcurrently(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := storetest.Postgres(t, "darts_o_mat_migrate_concurrently")

	const replicas = 4
	errs := make(chan error, replicas)
	for range replicas {
		go func() {
			db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
			defer db.Close()
			errs <- migrateSchema(ctx, db)
		}()
	}
	for range replicas {
		if err := <-errs; err != nil {
			t.Errorf("a replica: %v", err)
		}
	}

	db := bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
	defer db.Close()
	var names []string
	if err := db.NewSelect().Table("bun_migrations").Column("name").Scan(ctx, &names); err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if len(slices.Compact(slices.Clone(names))) != len(names) || len(names) == 0 {
		t.Errorf("recorded migrations = %q, want each once", names)
	}
}
