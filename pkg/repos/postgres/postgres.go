package postgres

import (
	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type Store struct {
	db *pg.DB
}

func NewStore() *Store {
	// docker run --name some-postgres -e POSTGRES_PASSWORD=mysecretpassword -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
	db := pg.Connect(&pg.Options{
		User:     "postgres",
		Password: "mysecretpassword",
		Database: "darts-o-mat",
	})

	models := []any{
		(*game.Game)(nil),
		(*game.PlayerState)(nil),
		(*player.Player)(nil),
		(*x01.GameX01)(nil),
		(*board.Board)(nil),
		(*throw.Dart)(nil),
	}

	for _, model := range models {
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			// Temp: true,
			// FKConstraints: ,
		})
		if err != nil {
			panic(err)
		}
	}

	return &Store{
		db: db,
	}
}
