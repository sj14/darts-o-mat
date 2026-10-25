package store

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/uptrace/bun/driver/pgdriver"
)

// busChannel is the PostgreSQL channel the replicas of the server notify each other on.
const busChannel = "darts_o_mat"

const (
	// busPing is how long the listener waits without a notification before it checks the
	// connection with one of its own (an empty payload).
	busPing = 15 * time.Second
	// busRetry is the pause before connecting again after the connection was lost.
	busRetry = time.Second
)

// Bus carries notifications between the replicas of the server with PostgreSQL's
// LISTEN/NOTIFY (service.Bus). The listener needs a session of its own, so it doesn't
// work through PgBouncer in transaction pooling.
type Bus struct {
	store *Store
}

// Bus returns the bus of a PostgreSQL database.
func (store *Store) Bus() *Bus {
	return &Bus{store: store}
}

// Publish sends the keys as one notification, separated by spaces. PostgreSQL delivers it
// once the statement is done, so the change has to be stored before.
func (b *Bus) Publish(ctx context.Context, keys ...string) error {
	return pgdriver.Notify(ctx, b.store.db, busChannel, strings.Join(keys, " "))
}

// Listen receives the notifications until ctx is done. After the connection was lost it
// connects again and calls missed, as notifications may have been sent meanwhile; also
// once at the start, which is harmless.
func (b *Bus) Listen(ctx context.Context, receive func(keys []string), missed func()) {
	for ctx.Err() == nil {
		err := b.listen(ctx, receive, missed)
		if ctx.Err() != nil {
			return
		}
		log.Printf("listen for changes: %v", err)
		select {
		case <-ctx.Done():
		case <-time.After(busRetry):
		}
	}
}

// listen listens on one connection until it fails or ctx is done. A connection that
// silently went away (e.g. a lost host) would block forever, so when nothing came for
// busPing it notifies itself, and if that doesn't come either the connection counts as
// lost.
func (b *Bus) listen(ctx context.Context, receive func(keys []string), missed func()) error {
	ln := pgdriver.NewListener(b.store.db)
	if err := ln.Listen(ctx, busChannel); err != nil {
		ln.Close()
		return err
	}
	missed()

	heard := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		// Closing the listener ends the Receive that waits.
		defer ln.Close()
		pinged := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-heard:
				pinged = false
			case <-time.After(busPing):
				if pinged {
					log.Print("listen for changes: no answer to a ping")
					return
				}
				pinged = true
				if err := pgdriver.Notify(ctx, b.store.db, busChannel, ""); err != nil {
					log.Printf("listen for changes: ping: %v", err)
				}
			}
		}
	}()

	for {
		_, payload, err := ln.Receive(ctx)
		if err != nil {
			return err
		}
		select {
		case heard <- struct{}{}:
		default:
		}
		if payload != "" {
			receive(strings.Fields(payload))
		}
	}
}
