package store

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
)

// AnonymizePlayer deletes a player: no name, no longer a guest of an account. Their
// games and darts stay, see player.Player.DeletedAt.
func (store *Store) AnonymizePlayer(ctx context.Context, id int64) error {
	return anonymize(ctx, store.db, "id = ?", id)
}

func anonymize(ctx context.Context, db bun.IDB, where string, arg any) error {
	_, err := db.NewUpdate().Model((*player.Player)(nil)).
		Set("name = ''").Set("guest_of = NULL").Set("deleted_at = ?", time.Now()).
		Where(where, arg).Where("deleted_at IS NULL").Exec(ctx)
	return err
}

// DeleteAccount deletes an account with what only it needs: its login, sessions,
// friendships and requests, boards with their camera tokens, the settings of its last
// games. Its player and its guests are anonymized (AnonymizePlayer). Games and tournaments stay as they are. The last admin
// can't be deleted (account.ErrLastAdmin).
func (store *Store) DeleteAccount(ctx context.Context, a account.Account) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := keepAdmin(ctx, tx, a.ID); err != nil {
			return err
		}
		if err := anonymize(ctx, tx, "guest_of = ?", a.ID); err != nil {
			return err
		}
		if err := anonymize(ctx, tx, "id = ?", a.PlayerID); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*account.Session)(nil)).Where("account_id = ?", a.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*account.Friendship)(nil)).Where("requester_id = ? OR addressee_id = ?", a.ID, a.ID).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*lastSettings)(nil)).Where("account_id = ?", a.ID).Exec(ctx); err != nil {
			return err
		}
		boards := tx.NewSelect().Model((*board.Board)(nil)).Column("id").Where("account_id = ?", a.ID)
		if _, err := tx.NewDelete().Model((*board.Token)(nil)).Where("board_id IN (?)", boards).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Model((*board.Board)(nil)).Where("account_id = ?", a.ID).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*account.Account)(nil)).Where("id = ?", a.ID).Exec(ctx)
		return err
	})
}
