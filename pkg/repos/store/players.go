package store

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/uptrace/bun"
)

func (store *Store) StorePlayer(ctx context.Context, p *player.Player) error {
	_, err := store.db.NewInsert().Model(p).Exec(ctx)
	return err
}

func (store *Store) ListPlayers(ctx context.Context) ([]player.Player, error) {
	result := []player.Player{}
	err := store.db.NewSelect().Model(&result).Order("id").Scan(ctx)
	return result, err
}

func (store *Store) GetPlayers(ctx context.Context, ids []int64) ([]player.Player, error) {
	result := []player.Player{}
	if len(ids) == 0 {
		return result, nil
	}
	err := store.db.NewSelect().Model(&result).Where("id IN (?)", bun.In(ids)).Scan(ctx)
	return result, err
}
