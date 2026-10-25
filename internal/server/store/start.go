package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// serverStart is when a server last started, one row (ID 1) for all replicas.
type serverStart struct {
	bun.BaseModel `bun:"table:server_start"`

	ID        int64     `bun:",pk"`
	StartedAt time.Time `bun:",notnull"`
}

// SetStarted stores that a server started at t.
func (store *Store) SetStarted(ctx context.Context, t time.Time) error {
	_, err := store.db.NewInsert().Model(&serverStart{ID: 1, StartedAt: t}).
		On("CONFLICT (id) DO UPDATE").Set("started_at = EXCLUDED.started_at").Exec(ctx)
	return err
}

// Started returns when a server last started, the zero time if none did yet.
func (store *Store) Started(ctx context.Context) (time.Time, error) {
	var s serverStart
	err := store.db.NewSelect().Model(&s).Where("id = 1").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	return s.StartedAt, err
}
