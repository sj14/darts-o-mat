package vision

import (
	"context"
	"errors"
	"image"
	"sync"
	"testing"
	"time"
)

// Next waits for a new frame, and stops waiting when its context is done, so a detection
// can be stopped while a camera delivers nothing.
func TestLiveNext(t *testing.T) {
	l := &Live{}
	l.cond = sync.NewCond(&l.mu)
	deliver := func() {
		l.mu.Lock()
		l.latest, l.seq = image.NewRGBA(image.Rect(0, 0, 1, 1)), l.seq+1
		l.cond.Broadcast()
		l.mu.Unlock()
	}

	go func() { time.Sleep(10 * time.Millisecond); deliver() }()
	if img, seq, err := l.Next(context.Background(), 0); err != nil || img == nil || seq != 1 {
		t.Fatalf("next frame: %v, %d, %v; want frame 1", img, seq, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(10 * time.Millisecond); cancel() }()
	done := make(chan error)
	go func() { _, _, err := l.Next(ctx, 1); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("stopped while waiting: %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Next doesn't stop waiting when its context is done")
	}
}
