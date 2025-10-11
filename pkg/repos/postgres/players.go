package postgres

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/player"
)

func (store *Store) StorePlayer(ctx context.Context, p *player.Player) error {
	_, err := store.db.Model(p).Insert()
	return err
}

func (store *Store) GetPlayers(ctx context.Context, ids []int) ([]player.Player, error) {
	var result []player.Player
	return result, store.db.Model(&result).WhereIn("id = ?", ids).Select()
}
