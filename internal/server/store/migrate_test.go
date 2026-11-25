package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
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
	(*cricketRow)(nil),
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
	(*loginFailure)(nil),
	(*lastSettings)(nil),
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
	if _, err := fromModels.NewCreateIndex().Model((*loginFailure)(nil)).Index("login_failures_key_failed_at").
		Column("key", "failed_at").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*account.Session)(nil)).Index("sessions_expires_at").
		Column("expires_at").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*throw.Dart)(nil)).Index("darts_game_id").
		Column("game_id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*game.Participant)(nil)).Index("game_players_board_id").
		Column("board_id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*game.Participant)(nil)).Index("game_players_player_id").
		Column("player_id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*stats.Result)(nil)).Index("game_stats_player_id").
		Column("player_id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*player.Rating)(nil)).Index("ratings_rated_id_rater_id_game_id").
		Column("rated_id", "rater_id", "game_id").Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := fromModels.NewCreateIndex().Model((*game.Game)(nil)).Index("games_rematch_of").
		Column("rematch_of").Unique().Exec(ctx); err != nil {
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

// A cancelled context doesn't stop the migrations halfway (a stop while the server starts):
// all of them run and are recorded, so the next start finds nothing to do.
func TestMigrationsIgnoreCancel(t *testing.T) {
	t.Parallel()
	sqldb, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	sqldb.SetMaxOpenConns(1)
	t.Cleanup(func() { sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := migrateSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	migrations := migrate.NewMigrations()
	files, err := fs.Sub(migrationFiles, "migrations/sqlite")
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Discover(files); err != nil {
		t.Fatal(err)
	}
	applied, err := migrate.NewMigrator(db, migrations).AppliedMigrations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := len(migrations.Sorted()); len(applied) != want {
		t.Errorf("%d migrations recorded, want %d", len(applied), want)
	}
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

// The migration that turned a bot's spread into tenths of a mm leaves a player who isn't a
// bot (0) and makes the others tenths, at least 4 mm. It runs on rows with the old values
// after the schema is there, as the migrator would have found them. The PostgreSQL run needs
// storetest.PostgresEnv.
func TestBotSpreadTenthsMigration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		dir  string
		open func(t *testing.T) *bun.DB
	}{
		{"sqlite", func(t *testing.T) *bun.DB {
			sqldb, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
			if err != nil {
				t.Fatal(err)
			}
			sqldb.SetMaxOpenConns(1)
			return bun.NewDB(sqldb, sqlitedialect.New())
		}},
		{"postgres", func(t *testing.T) *bun.DB {
			dsn := storetest.Postgres(t, "darts_o_mat_bot_spread_tenths")
			return bun.NewDB(sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn))), pgdialect.New())
		}},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			db := tt.open(t)
			t.Cleanup(func() { db.Close() })
			if err := migrateSchema(ctx, db); err != nil {
				t.Fatal(err)
			}
			for player, spread := range []int{0, 1, 20} {
				if _, err := db.ExecContext(ctx, `INSERT INTO game_players (game_id, player_id, place, status, bot_spread) VALUES (1, ?, 0, 'accepted', ?)`, player, spread); err != nil {
					t.Fatal(err)
				}
			}
			up, err := migrationFiles.ReadFile("migrations/" + tt.dir + "/since-v0.0.4/20261123000000_bot_spread_tenths.tx.up.sql")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, string(up)); err != nil {
				t.Fatal(err)
			}
			var got []int
			if err := db.NewRaw(`SELECT bot_spread FROM game_players ORDER BY player_id`).Scan(ctx, &got); err != nil {
				t.Fatal(err)
			}
			if want := []int{0, 40, 200}; !slices.Equal(got, want) {
				t.Errorf("spreads %v, want %v", got, want)
			}
		})
	}
}
