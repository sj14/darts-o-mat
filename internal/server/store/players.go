package store

import (
	"context"
	"slices"

	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
)

func (store *Store) StorePlayer(ctx context.Context, p *player.Player) error {
	_, err := store.db.NewInsert().Model(p).Exec(ctx)
	return err
}

// SetPlayerColor stores a player's accent color, empty for the default.
func (store *Store) SetPlayerColor(ctx context.Context, playerID int64, color string) error {
	_, err := store.db.NewUpdate().Model((*player.Player)(nil)).Set("color = ?", color).Where("id = ?", playerID).Exec(ctx)
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

// GetPlayers returns the players with the ids, in their order; ids without a player
// are left out.
func (store *Store) GetPlayers(ctx context.Context, ids []int64) ([]player.Player, error) {
	result := []player.Player{}
	if len(ids) == 0 {
		return result, nil
	}
	var found []player.Player
	if err := store.db.NewSelect().Model(&found).Where("id IN (?)", bun.In(ids)).Scan(ctx); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if i := slices.IndexFunc(found, func(p player.Player) bool { return p.ID == id }); i >= 0 {
			result = append(result, found[i])
		}
	}
	return result, nil
}

// PlayerAccounts returns the account of each player, by player ID: its own, or the one a
// guest belongs to. Deleted players belong to nobody and are left out.
func (store *Store) PlayerAccounts(ctx context.Context, playerIDs []int64) (map[int64]int64, error) {
	accounts := map[int64]int64{}
	if len(playerIDs) == 0 {
		return accounts, nil
	}
	var rows []struct {
		PlayerID  int64
		AccountID int64
	}
	err := store.db.NewSelect().Model((*player.Player)(nil)).
		ColumnExpr("player.id AS player_id, COALESCE(player.guest_of, account.id) AS account_id").
		Join("LEFT JOIN accounts AS account ON account.player_id = player.id").
		Where("player.id IN (?)", bun.In(playerIDs)).
		Where("COALESCE(player.guest_of, account.id) IS NOT NULL").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		accounts[row.PlayerID] = row.AccountID
	}
	return accounts, nil
}
