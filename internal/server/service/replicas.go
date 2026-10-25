package service

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/store"
)

// What the replicas of the server share besides the games, one interface per concern, so
// that each could be served by another service later (e.g. Redis or NATS) without the
// others. So far the database (store.Store) is the only implementation of each; the live
// updates have their own, Bus. Options leaves them nil to use the database.

// Locker orders the changes of games and seats across the replicas (Service.lock). It has
// to be released when its holder dies, so it stays in PostgreSQL: a lock elsewhere would
// need an expiry, and a replica that stalls longer could write at the same time as
// another one.
type Locker interface {
	// Lock waits until the lock is free or ctx is done; call unlock when the change is done.
	Lock(ctx context.Context) (unlock func(), err error)
}

// LoginFailures counts the failed logins per key (a username or a network, see
// loginLimiter).
type LoginFailures interface {
	// CountLoginFailures returns how many failed logins the key had after since.
	CountLoginFailures(ctx context.Context, key string, since time.Time) (int, error)
	// AddLoginFailure counts a failed login of the key at the time, and forgets the
	// failures of all keys that are older than forget.
	AddLoginFailure(ctx context.Context, key string, at, forget time.Time) error
	// ResetLoginFailures forgets the failed logins of the key.
	ResetLoginFailures(ctx context.Context, key string) error
}

// LastSeen keeps when each account was last online, for the presence of friends.
type LastSeen interface {
	// SetLastSeen stores when the account was last online.
	SetLastSeen(ctx context.Context, accountID int64, t time.Time) error
	// LastSeen returns when the accounts were last online, by account ID; an account
	// that never was is missing.
	LastSeen(ctx context.Context, accountIDs []int64) (map[int64]time.Time, error)
}

// ServerStart keeps when a server last started, one time for all replicas (see
// GameView.TurnDeadline).
type ServerStart interface {
	// SetStarted stores that a server started at t.
	SetStarted(ctx context.Context, t time.Time) error
	// Started returns when a server last started, the zero time if none did yet.
	Started(ctx context.Context) (time.Time, error)
}

// The database serves them all.
var (
	_ Locker        = (*store.Store)(nil)
	_ LoginFailures = (*store.Store)(nil)
	_ LastSeen      = (*store.Store)(nil)
	_ ServerStart   = (*store.Store)(nil)
)
