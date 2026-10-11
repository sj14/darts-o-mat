package store

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
)

func (store *Store) CreateBoard(ctx context.Context, b *board.Board) error {
	_, err := store.db.NewInsert().Model(b).Returning("*").Exec(ctx)
	return err
}

func (store *Store) ListBoards(ctx context.Context, accountID int64) ([]board.Board, error) {
	boards := []board.Board{}
	err := store.db.NewSelect().Model(&boards).Where("account_id = ?", accountID).Order("id").Scan(ctx)
	return boards, err
}

func (store *Store) GetBoard(ctx context.Context, id int64) (board.Board, error) {
	var b board.Board
	err := store.db.NewSelect().Model(&b).Where("id = ?", id).Scan(ctx)
	return b, notFound(err)
}

// DeleteBoard deletes a board of the account, ErrNotFound if the account has no such board.
func (store *Store) DeleteBoard(ctx context.Context, accountID, id int64) error {
	res, err := store.db.NewDelete().Model((*board.Board)(nil)).Where("id = ?", id).Where("account_id = ?", accountID).Exec(ctx)
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
