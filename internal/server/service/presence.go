package service

import (
	"context"
	"log"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
)

// Presence is what a friend sees of an account: whether it is online and playing.
type Presence string

const (
	PresenceOffline Presence = "offline"
	PresenceOnline  Presence = "online"
	// PresencePlaying is online with a seat in a running game.
	PresencePlaying Presence = "playing"
)

// onlineFor is how long an account counts as online after its last request.
const onlineFor = 5 * time.Minute

// seenEvery is how often an account's last request is stored at most (Options.LastSeen),
// and how often an open live stream stores it.
const seenEvery = time.Minute

// seen notes a request of the account (Authenticate) in Options.LastSeen, so every replica
// knows it, but only if the stored time is older than seenEvery: one write a minute, not
// one per request. The stored time comes with the account the session loaded
// (account.Account.LastSeenAt, the database's column), so this costs no query; with
// another LastSeen it stays empty and every request writes.
func (s *Service) seen(ctx context.Context, a account.Account) {
	now := s.now()
	if a.LastSeenAt != nil && now.Sub(*a.LastSeenAt) < seenEvery {
		return
	}
	if err := s.opts.LastSeen.SetLastSeen(ctx, a.ID, now); err != nil {
		log.Printf("note that account %d was seen: %v", a.ID, err)
	}
}

// Watching keeps the account online while a page streams live updates to it (the game
// and tournament pages make no requests in between): it stores the time every seenEvery
// (Service.watchEvery in tests) and when the stream ends. Call stop when it ends.
func (s *Service) Watching(acc account.Account) (stop func()) {
	stopped, done := make(chan struct{}), make(chan struct{})
	note := func() {
		if err := s.opts.LastSeen.SetLastSeen(context.Background(), acc.ID, s.now()); err != nil {
			log.Printf("note that account %d was seen: %v", acc.ID, err)
		}
	}
	note()
	go func() {
		defer close(done)
		tick := time.NewTicker(s.watchEvery)
		defer tick.Stop()
		for {
			select {
			case <-stopped:
				note() // online for a while after the page closed
				return
			case <-tick.C:
				note()
			}
		}
	}()
	return func() {
		close(stopped)
		<-done
	}
}

// setPresence fills in the presence of the friends: offline if invisible or not online,
// else playing if their own player has a seat in a running game.
func (s *Service) setPresence(ctx context.Context, friends []Friend) error {
	var ids []int64
	for _, f := range friends {
		if !f.invisible {
			ids = append(ids, f.AccountID)
		}
	}
	seen, err := s.opts.LastSeen.LastSeen(ctx, ids)
	if err != nil {
		return err
	}
	var online []int64
	for i, f := range friends {
		friends[i].Presence = PresenceOffline
		if last, ok := seen[f.AccountID]; ok && s.now().Sub(last) < onlineFor { // not asked for the invisible
			friends[i].Presence = PresenceOnline
			online = append(online, f.PlayerID)
		}
	}
	playing, err := s.store.PlayingPlayers(ctx, online)
	if err != nil {
		return err
	}
	for i, f := range friends {
		if slices.Contains(playing, f.PlayerID) {
			friends[i].Presence = PresencePlaying
		}
	}
	return nil
}
