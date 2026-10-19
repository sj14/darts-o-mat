// Package notify tells subscribers that something changed, e.g. to push the new state
// to open pages.
package notify

import "sync"

// Signal tells its subscribers that something changed. Changes that happen while a
// subscriber is busy are merged into one notification, so a slow subscriber never
// blocks Notify. The zero value is ready to use.
type Signal struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// Subscribe returns a channel that receives a value after every change, and a function
// to stop the subscription.
func (s *Signal) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	if s.subs == nil {
		s.subs = map[chan struct{}]struct{}{}
	}
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	return ch, func() {
		s.mu.Lock()
		delete(s.subs, ch)
		s.mu.Unlock()
	}
}

// Notify tells the subscribers about a change.
func (s *Signal) Notify() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default: // a notification is already pending
		}
	}
}

func (s *Signal) empty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subs) == 0
}

// Hub is a Signal per key, e.g. per game; a key without subscribers takes no memory.
// The zero value is ready to use.
type Hub[K comparable] struct {
	mu      sync.Mutex
	signals map[K]*Signal
}

// Subscribe subscribes to the changes of a key, see Signal.Subscribe.
func (h *Hub[K]) Subscribe(key K) (<-chan struct{}, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.signals == nil {
		h.signals = map[K]*Signal{}
	}
	s := h.signals[key]
	if s == nil {
		s = &Signal{}
		h.signals[key] = s
	}
	ch, stop := s.Subscribe()
	return ch, func() {
		stop()
		h.mu.Lock()
		defer h.mu.Unlock()
		if s.empty() && h.signals[key] == s {
			delete(h.signals, key)
		}
	}
}

// Notify tells the subscribers of a key about a change.
func (h *Hub[K]) Notify(key K) {
	h.mu.Lock()
	s := h.signals[key]
	h.mu.Unlock()
	if s != nil {
		s.Notify()
	}
}
