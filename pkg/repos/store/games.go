package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

var ErrNotFound = errors.New("not found")

func (store *Store) CreateGame(ctx context.Context, g *game.Game) error {
	_, err := store.db.NewInsert().Model(g).Returning("*").Exec(ctx)
	return err
}

func (store *Store) GetGame(ctx context.Context, id int64) (game.Game, error) {
	var g game.Game
	err := store.db.NewSelect().Model(&g).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return g, ErrNotFound
	}
	return g, err
}

func (store *Store) AddDart(ctx context.Context, d *throw.Dart) error {
	_, err := store.db.NewInsert().Model(d).Exec(ctx)
	return err
}

// ListDarts returns the darts of a game in the order they were thrown.
func (store *Store) ListDarts(ctx context.Context, gameID int64) ([]throw.Dart, error) {
	darts := []throw.Dart{}
	err := store.db.NewSelect().Model(&darts).Where("game_id = ?", gameID).Order("id").Scan(ctx)
	return darts, err
}

// DeleteLastDart removes the most recent dart of a game, ErrNotFound if there is none.
func (store *Store) DeleteLastDart(ctx context.Context, gameID int64) error {
	last := store.db.NewSelect().Model((*throw.Dart)(nil)).ColumnExpr("MAX(id)").Where("game_id = ?", gameID)
	res, err := store.db.NewDelete().Model((*throw.Dart)(nil)).Where("id = (?)", last).Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
