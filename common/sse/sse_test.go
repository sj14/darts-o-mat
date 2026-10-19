package sse

import (
	"bufio"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Stream sends the rendered data as an event after each change, every line in its own
// data field, and ends when render fails.
func TestStream(t *testing.T) {
	changes := make(chan struct{}, 1)
	renders := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Stream(w, r, "board", changes, func() (string, error) {
			renders++
			if renders > 1 {
				return "", errors.New("gone")
			}
			return "<div>\n</div>", nil
		})
	}))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("content type %q", ct)
	}
	changes <- struct{}{}
	r := bufio.NewReader(resp.Body)
	var got []string
	for range 4 {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, line)
	}
	if want := "event: board\ndata: <div>\ndata: </div>\n\n"; strings.Join(got, "") != want {
		t.Errorf("sent %q, want %q", strings.Join(got, ""), want)
	}
	changes <- struct{}{}
	if rest, _ := r.ReadString('\n'); rest != "" {
		t.Errorf("sent %q after render failed, want the stream to end", rest)
	}
}
