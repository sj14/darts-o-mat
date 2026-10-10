package store

import (
	"context"
	"database/sql"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *bun.DB
}

// NewPostgres connects to PostgreSQL, e.g. with
// dsn "postgres://postgres:mysecretpassword@localhost:5432/darts-o-mat?sslmode=disable".
//
// docker run --name some-postgres -e POSTGRES_PASSWORD=mysecretpassword -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
func NewPostgres(ctx context.Context, dsn string) (*Store, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	return newStore(ctx, bun.NewDB(sqldb, pgdialect.New()))
}

// NewSQLite opens the SQLite database at dsn, e.g. "file:darts-o-mat.db".
func NewSQLite(ctx context.Context, dsn string) (*Store, error) {
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows only one writer at a time.
	sqldb.SetMaxOpenConns(1)
	return newStore(ctx, bun.NewDB(sqldb, sqlitedialect.New()))
}

func newStore(ctx context.Context, db *bun.DB) (*Store, error) {
	models := []any{
		(*game.Game)(nil),
		(*player.Player)(nil),
		(*board.Board)(nil),
		(*throw.Dart)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().Model(model).IfNotExists().Exec(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &Store{
		db: db,
	}, nil
}
