package game

import (
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

type Game struct {
	// gorm.Model
	ID     uint64
	Throws []throw.Dart `pg:"rel:has-many"`
}

type PlayerState struct {
	// gorm.Model
	ID   uint64
	Game Game
	player.Player
	Board  board.Board
	Throws []throw.Dart
}
