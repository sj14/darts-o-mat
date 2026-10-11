package service

import "sync"

// hub tells subscribers when a game changed, e.g. to push the new board to open game pages.
type hub struct {
	mu   sync.Mutex
	subs map[int64]map[chan struct{}]struct{}
}

// Subscribe returns a channel that receives a value whenever the game changes,
// and a function to stop the subscription. Changes that happen while the receiver is
// busy are merged into one notification.
func (s *Service) Subscribe(gameID int64) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.hub.mu.Lock()
	if s.hub.subs == nil {
		s.hub.subs = map[int64]map[chan struct{}]struct{}{}
	}
	if s.hub.subs[gameID] == nil {
		s.hub.subs[gameID] = map[chan struct{}]struct{}{}
	}
	s.hub.subs[gameID][ch] = struct{}{}
	s.hub.mu.Unlock()

	return ch, func() {
		s.hub.mu.Lock()
		delete(s.hub.subs[gameID], ch)
		if len(s.hub.subs[gameID]) == 0 {
			delete(s.hub.subs, gameID)
		}
		s.hub.mu.Unlock()
	}
}

// notify tells the game's subscribers that it changed.
func (s *Service) notify(gameID int64) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	for ch := range s.hub.subs[gameID] {
		select {
		case ch <- struct{}{}:
		default: // a notification is already pending
		}
	}
}
