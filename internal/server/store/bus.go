package store

import (
	"context"
	"errors"
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
	// busStart is how often the listener notifies itself until the first notification
	// arrives, and busStartTries how often before it gives up on the connection.
	busStart      = 100 * time.Millisecond
	busStartTries = 100
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

// listen listens on one connection until it fails or ctx is done.
//
// The driver sends LISTEN without waiting for its answer, so a notification sent right
// after it may still be lost: the listener only counts as listening once a notification
// arrived, and it notifies itself until one does. Only then it calls missed, so whatever
// changed before is covered by it and everything after is received.
func (b *Bus) listen(ctx context.Context, receive func(keys []string), missed func()) error {
	ln := pgdriver.NewListener(b.store.db)
	if err := ln.Listen(ctx, busChannel); err != nil {
		ln.Close()
		return err
	}
	heard := make(chan struct{}, 1)
	done := make(chan struct{})
	defer close(done)
	go func() {
		// Closing the listener ends the Receive that waits.
		defer ln.Close()
		if err := b.ping(ctx, heard, done); err != nil {
			log.Printf("listen for changes: %v", err)
		}
	}()

	listening := false
	for {
		_, payload, err := ln.Receive(ctx)
		if err != nil {
			return err
		}
		select {
		case heard <- struct{}{}:
		default:
		}
		if !listening {
			listening = true
			missed()
		}
		if payload != "" {
			receive(strings.Fields(payload))
		}
	}
}

// ping notifies the listener (an empty payload) until it heard something, every busStart,
// and later whenever nothing came for busPing: a connection that silently went away (e.g.
// a lost host) would wait forever. It returns, and the connection counts as lost, when the
// listener doesn't hear the pings, or when ctx is done or the listener stopped (done).
func (b *Bus) ping(ctx context.Context, heard, done <-chan struct{}) error {
	listening, pinged := false, 0
	for {
		wait := busPing
		if !listening {
			wait = busStart
		}
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			return nil
		case <-heard:
			listening, pinged = true, 0
			continue
		case <-time.After(wait):
		}
		if (listening && pinged > 0) || pinged >= busStartTries {
			return errors.New("no answer to a ping")
		}
		pinged++
		if err := pgdriver.Notify(ctx, b.store.db, busChannel, ""); err != nil {
			log.Printf("listen for changes: ping: %v", err)
		}
	}
}
