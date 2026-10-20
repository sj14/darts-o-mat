package camapp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// fakeServer answers like darts-o-mat, after down failed requests (503 like a proxy
// without a server behind it); it records what reached it.
type fakeServer struct {
	mu       sync.Mutex
	down     int
	status   int // the answer once up, 0 for 201 with game 7
	received []string
}

func (f *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.down > 0 {
		f.down--
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	f.received = append(f.received, r.URL.Path)
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`{"error":"no running game on this board"}`))
		return
	}
	_, _ = w.Write([]byte(`{"game":{"id":7}}`))
}

type outcome struct {
	event   int
	game    int64
	err     error
	sending bool
}

// testReporter returns a reporter to f with short pauses and the outcomes it reports.
func testReporter(t *testing.T, f *fakeServer) (*reporter, <-chan outcome) {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	out := make(chan outcome, 100)
	r := newReporter("board", newServer(srv.URL, "secret"), func(rep report, game int64, err error, sending bool) {
		out <- outcome{rep.event, game, err, sending}
	})
	r.firstPause = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go r.run(ctx)
	return r, out
}

// final returns the next outcome that isn't a failed try.
func final(t *testing.T, out <-chan outcome) outcome {
	t.Helper()
	for {
		select {
		case o := <-out:
			if !o.sending {
				return o
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no outcome")
		}
	}
}

// While the server is down the reports wait and are sent again, in order.
func TestReporterSendsAgain(t *testing.T) {
	f := &fakeServer{down: 3}
	r, out := testReporter(t, f)
	now := time.Now()
	r.add(report{event: 1, at: now, dart: dartboard.Hit{Number: 20, Segment: dartboard.Triple}})
	r.add(report{event: 2, at: now, takeout: true})
	r.add(report{event: 3, at: now, dart: dartboard.Hit{Number: 5, Segment: dartboard.Single}})
	for want := 1; want <= 3; want++ {
		if o := final(t, out); o.event != want || o.err != nil || o.game != 7 {
			t.Fatalf("outcome %+v, want event %d in game 7", o, want)
		}
	}
	want := []string{"/api/board/darts", "/api/board/takeout", "/api/board/darts"}
	if len(f.received) != len(want) {
		t.Fatalf("received %v, want %v", f.received, want)
	}
	for i := range want {
		if f.received[i] != want[i] {
			t.Errorf("request %d: %s, want %s", i, f.received[i], want[i])
		}
	}
}

// An answer of the server is final: a refused dart isn't sent again.
func TestReporterDoesntRepeatRefusals(t *testing.T) {
	f := &fakeServer{status: http.StatusConflict}
	r, out := testReporter(t, f)
	r.add(report{event: 1, at: time.Now(), dart: dartboard.Hit{Number: 20, Segment: dartboard.Single}})
	if o := final(t, out); o.err == nil || errors.Is(o.err, errUnreachable) {
		t.Errorf("outcome %+v, want the server's refusal", o)
	}
	if len(f.received) != 1 {
		t.Errorf("sent %d times, want once", len(f.received))
	}
}

// A report that couldn't be sent within giveUpAfter of its detection is dropped; a refused
// connection (the server restarts) is tried again until then.
func TestReporterGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listens there anymore
	out := make(chan outcome, 100)
	r := newReporter("board", newServer(srv.URL, "secret"), func(rep report, game int64, err error, sending bool) {
		out <- outcome{rep.event, game, err, sending}
	})
	r.firstPause, r.giveUpAfter = time.Millisecond, 20*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.run(ctx)
	r.add(report{event: 1, at: time.Now(), dart: dartboard.Hit{Number: 20, Segment: dartboard.Single}})
	if o := final(t, out); !errors.Is(o.err, errGaveUp) || !errors.Is(o.err, errUnreachable) {
		t.Errorf("outcome %+v, want given up as unreachable", o)
	}
}

// After a timeout the server may have entered the dart: it isn't sent again, which could
// count it twice.
func TestReporterDoesntRepeatAfterTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release // answers only after the client gave up
	}))
	defer srv.Close()
	defer close(release) // first, so Close doesn't wait for the handler
	out := make(chan outcome, 100)
	s := newServer(srv.URL, "secret")
	s.client.Timeout = 20 * time.Millisecond
	r := newReporter("board", s, func(rep report, game int64, err error, sending bool) {
		out <- outcome{rep.event, game, err, sending}
	})
	r.firstPause = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go r.run(ctx)
	r.add(report{event: 1, at: time.Now(), dart: dartboard.Hit{Number: 20, Segment: dartboard.Single}})
	if o := final(t, out); o.err == nil || errors.Is(o.err, errUnreachable) {
		t.Errorf("outcome %+v, want the timeout, not sent again", o)
	}
}
