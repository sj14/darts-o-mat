package sqlite

import (
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SQLiteStore struct {
	db *gorm.DB
}

func NewSQLiteStore() *SQLiteStore {
	// os.Remove("./test.db")

	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	err = db.AutoMigrate(
		&board.Board{},
		&player.Player{},
		&game.Game{},
		&game.PlayerState{},
		&x01.X01PlayerState{},
		&x01.GameX01{},
		&throw.Dart{},
	)
	if err != nil {
		panic("failed to migrate database")
	}

	return &SQLiteStore{
		db: db,
	}
}
