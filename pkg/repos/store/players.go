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

// ListPlayers returns the player of an account followed by its guests.
func (store *Store) ListPlayers(ctx context.Context, accountID, playerID int64) ([]player.Player, error) {
	result := []player.Player{}
	err := store.db.NewSelect().Model(&result).
		Where("id = ?", playerID).
		WhereOr("guest_of = ?", accountID).
		OrderExpr("guest_of IS NOT NULL, id").
		Scan(ctx)
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
