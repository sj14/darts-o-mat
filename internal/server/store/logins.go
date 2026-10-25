package store

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// loginFailure is a failed login, by the key it counts for (a username or a network, the
// service tells them apart).
type loginFailure struct {
	bun.BaseModel `bun:"table:login_failures"`

	Key      string    `bun:",notnull"`
	FailedAt time.Time `bun:",notnull"`
}

// CountLoginFailures returns how many failed logins the key had after since.
func (store *Store) CountLoginFailures(ctx context.Context, key string, since time.Time) (int, error) {
	n, err := store.db.NewSelect().Model((*loginFailure)(nil)).Where("key = ?", key).Where("failed_at > ?", since).Count(ctx)
	return int(n), err
}

// AddLoginFailure stores a failed login of the key at the time, and forgets the failures
// of all keys that are older than forget.
func (store *Store) AddLoginFailure(ctx context.Context, key string, at, forget time.Time) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*loginFailure)(nil)).Where("failed_at <= ?", forget).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(&loginFailure{Key: key, FailedAt: at}).Exec(ctx)
		return err
	})
}

// ResetLoginFailures forgets the failed logins of the key.
func (store *Store) ResetLoginFailures(ctx context.Context, key string) error {
	_, err := store.db.NewDelete().Model((*loginFailure)(nil)).Where("key = ?", key).Exec(ctx)
	return err
}
