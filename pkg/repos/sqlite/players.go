package sqlite

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"gorm.io/gorm"
)

func (store *SQLiteStore) StorePlayer(ctx context.Context, p *player.Player) error {
	return gorm.G[player.Player](store.db).Create(ctx, p)

}

func (store *SQLiteStore) GetPlayers(ctx context.Context, ids []int) ([]player.Player, error) {
	return gorm.G[player.Player](store.db).Where("id IN ?", ids).Find(ctx)
}
