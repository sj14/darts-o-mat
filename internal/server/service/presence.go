package service

import (
	"context"
	"slices"
	"sync"
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

// presence keeps when each account was last seen and how many live streams it has open,
// in memory like the live updates (one process; after a restart everyone is offline until
// their next request).
type presence struct {
	mu      sync.Mutex
	seen    map[int64]time.Time
	streams map[int64]int
}

// seen notes a request of the account (Authenticate).
func (s *Service) seen(accountID int64) {
	s.presence.mu.Lock()
	defer s.presence.mu.Unlock()
	s.presence.seen[accountID] = s.now()
}

// Watching keeps the account online while a page streams live updates to it (the game
// and tournament pages make no requests in between); call stop when the stream ends.
func (s *Service) Watching(acc account.Account) (stop func()) {
	s.presence.mu.Lock()
	defer s.presence.mu.Unlock()
	s.presence.streams[acc.ID]++
	return func() {
		s.presence.mu.Lock()
		defer s.presence.mu.Unlock()
		if s.presence.streams[acc.ID]--; s.presence.streams[acc.ID] <= 0 {
			delete(s.presence.streams, acc.ID)
		}
		s.presence.seen[acc.ID] = s.now() // online for a while after the page closed
	}
}

// online reports whether the account made a request lately or has a live stream open.
func (s *Service) online(accountID int64) bool {
	s.presence.mu.Lock()
	defer s.presence.mu.Unlock()
	return s.presence.streams[accountID] > 0 || s.now().Sub(s.presence.seen[accountID]) < onlineFor
}

// setPresence fills in the presence of the friends: offline if invisible or not online,
// else playing if their own player has a seat in a running game.
func (s *Service) setPresence(ctx context.Context, friends []Friend) error {
	var online []int64
	for i, f := range friends {
		friends[i].Presence = PresenceOffline
		if !f.invisible && s.online(f.AccountID) {
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
