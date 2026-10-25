package store

import (
	"context"
	"database/sql/driver"
	"log"

	"github.com/uptrace/bun/dialect"
)

// changeLock is the key of the PostgreSQL advisory lock that orders the changes of games
// and seats across the replicas of the server (service.Service.lock).
const changeLock = 0x6461727473 + 1

// Lock takes the lock that orders the changes of games and seats across the replicas: a
// PostgreSQL advisory lock on a connection of its own, which is released when the
// connection ends, also when the process dies. It waits until the lock is free or ctx is
// done. SQLite is one process, there it locks nothing.
func (store *Store) Lock(ctx context.Context) (unlock func(), err error) {
	if store.db.Dialect().Name() != dialect.PG {
		return func() {}, nil
	}
	conn, err := store.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	// A connection whose lock may still be held mustn't go back to the pool: discarding it
	// ends its session and so the lock.
	discard := func() {
		conn.Raw(func(any) error { return driver.ErrBadConn })
		conn.Close()
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock(?)", changeLock); err != nil {
		discard()
		return nil, err
	}
	return func() {
		if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock(?)", changeLock); err != nil {
			log.Printf("unlock: %v", err)
			discard()
			return
		}
		conn.Close()
	}, nil
}
