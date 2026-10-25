package profiling

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"/debug/pprof/":                  "goroutine",
		"/debug/pprof/goroutine?debug=1": "TestHandler",
	} {
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d, want 200 with %q", path, rec.Code, want)
		}
	}
}

// Without an address nothing is served; a taken one is an error.
func TestStart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Start(ctx, ""); err != nil {
		t.Errorf("no address: %v", err)
	}
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if err := Start(ctx, srv.Listener.Addr().String()); err == nil {
		t.Error("an address in use: no error")
	}
}
