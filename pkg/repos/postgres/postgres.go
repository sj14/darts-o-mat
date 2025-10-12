package postgres

import (
	"context"
	"database/sql"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type Store struct {
	db *bun.DB
}

func NewStore(ctx context.Context) *Store {
	// docker run --name some-postgres -e POSTGRES_PASSWORD=mysecretpassword -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
	sqldb := sql.OpenDB(pgdriver.NewConnector(
		pgdriver.WithDSN("postgres://postgres:mysecretpassword@localhost:5432/darts-o-mat?sslmode=disable"),
	))
	db := bun.NewDB(sqldb, pgdialect.New())

	models := []any{
		(*game.Game)(nil),
		(*game.PlayerState)(nil),
		(*player.Player)(nil),
		(*x01.GameX01)(nil),
		(*board.Board)(nil),
		(*throw.Dart)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().Model(model).Exec(ctx)
		if err != nil {
			panic(err)
		}
	}

	return &Store{
		db: db,
	}
}
