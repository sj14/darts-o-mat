package store

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/player"
)

func (store *Store) StorePlayer(ctx context.Context, p *player.Player) error {
	_, err := store.db.NewInsert().Model(p).Exec(ctx)
	return err
}

func (store *Store) GetPlayers(ctx context.Context, ids []int) ([]player.Player, error) {
	var result []player.Player
	_, err := store.db.NewSelect().Model(&result).Where("id = ?", ids).Exec(ctx)
	return result, err
}
