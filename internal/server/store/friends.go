package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/uptrace/bun"
)

// GetFriendship returns the friendship between two accounts in either direction.
func (store *Store) GetFriendship(ctx context.Context, a, b int64) (account.Friendship, error) {
	var f account.Friendship
	err := store.db.NewSelect().Model(&f).
		WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("requester_id = ? AND addressee_id = ?", a, b)
		}).
		WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("requester_id = ? AND addressee_id = ?", b, a)
		}).
		Scan(ctx)
	return f, notFound(err)
}

func (store *Store) CreateFriendship(ctx context.Context, f *account.Friendship) error {
	_, err := store.db.NewInsert().Model(f).Exec(ctx)
	return err
}

// AcceptFriendship accepts a pending request, ErrNotFound if there is none.
func (store *Store) AcceptFriendship(ctx context.Context, requesterID, addresseeID int64) error {
	res, err := store.db.NewUpdate().Model((*account.Friendship)(nil)).
		Set("status = ?", account.FriendshipAccepted).
		Where("requester_id = ?", requesterID).
		Where("addressee_id = ?", addresseeID).
		Where("status = ?", account.FriendshipPending).
		Exec(ctx)
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

// DeleteFriendship deletes the friendship or request between two accounts in either direction.
func (store *Store) DeleteFriendship(ctx context.Context, a, b int64) error {
	_, err := store.db.NewDelete().Model((*account.Friendship)(nil)).
		WhereGroup(" OR ", func(q *bun.DeleteQuery) *bun.DeleteQuery {
			return q.Where("requester_id = ? AND addressee_id = ?", a, b)
		}).
		WhereGroup(" OR ", func(q *bun.DeleteQuery) *bun.DeleteQuery {
			return q.Where("requester_id = ? AND addressee_id = ?", b, a)
		}).
		Exec(ctx)
	return err
}

// ListFriendships returns all friendships and requests of an account.
func (store *Store) ListFriendships(ctx context.Context, accountID int64) ([]account.Friendship, error) {
	friendships := []account.Friendship{}
	err := store.db.NewSelect().Model(&friendships).
		Where("requester_id = ?", accountID).
		WhereOr("addressee_id = ?", accountID).
		Order("created_at").
		Scan(ctx)
	return friendships, err
}

func (store *Store) GetAccounts(ctx context.Context, ids []int64) ([]account.Account, error) {
	accounts := []account.Account{}
	if len(ids) == 0 {
		return accounts, nil
	}
	err := store.db.NewSelect().Model(&accounts).Where("id IN (?)", bun.In(ids)).Scan(ctx)
	return accounts, err
}
