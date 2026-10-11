package store

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/uptrace/bun"
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
	_, err = store.db.NewDelete().Model((*board.Token)(nil)).Where("board_id = ?", id).Exec(ctx)
	return err
}

// SetBoardToken stores the hash of a board's token, replacing an older one.
func (store *Store) SetBoardToken(ctx context.Context, boardID int64, tokenHash string) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*board.Token)(nil)).Where("board_id = ?", boardID).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(&board.Token{BoardID: boardID, TokenHash: tokenHash}).Exec(ctx)
		return err
	})
}

// HasBoardToken reports whether a token was created for the board.
func (store *Store) HasBoardToken(ctx context.Context, boardID int64) (bool, error) {
	return store.db.NewSelect().Model((*board.Token)(nil)).Where("board_id = ?", boardID).Exists(ctx)
}

// GetBoardByToken returns the board a token hash belongs to.
func (store *Store) GetBoardByToken(ctx context.Context, tokenHash string) (board.Board, error) {
	var t board.Token
	if err := store.db.NewSelect().Model(&t).Where("token_hash = ?", tokenHash).Scan(ctx); err != nil {
		return board.Board{}, notFound(err)
	}
	return store.GetBoard(ctx, t.BoardID)
}
