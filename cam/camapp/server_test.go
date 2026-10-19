package camapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/common/throw"
)

// A dart the server refuses, e.g. because the player who is up throws on another
// board, comes back as an error with the server's reason, which the event list shows.
func TestServerReportsRefusedDarts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/board/darts" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"the player who is up doesn't throw on this board"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"game":{"id":7}}`))
	}))
	defer srv.Close()

	s := newServer(srv.URL+"/", "secret")
	_, err := s.dart(context.Background(), throw.Dart{Number: 20, Segment: throw.Single})
	if err == nil || !strings.Contains(err.Error(), "doesn't throw on this board") {
		t.Errorf("refused dart: got %v, want the server's reason", err)
	}
	if id, err := s.takeout(context.Background()); err != nil || id != 7 {
		t.Errorf("takeout: game %d, %v; want game 7", id, err)
	}
}
