package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	tests := []struct {
		latest, current string
		want            bool
	}{
		{"1.2.4", "1.2.3", true},
		{"1.3.0", "1.2.9", true},
		{"2.0.0", "1.9.9", true},
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.4", false},
		{"1.10.0", "1.9.0", true}, // numbers, not text
		{"1.2.3", "1.2.3-rc.1", true},
		{"1.2.3", "v1.2.2", true},
		{"1.2.3", "9a8b7c6", false}, // not a release: no hint
		{"1.2.3", "9a8b7c6-dirty", false},
		{"1.2.3", "", false},
		{"", "1.2.3", false},
	}
	for _, tt := range tests {
		if got := newer(tt.latest, tt.current); got != tt.want {
			t.Errorf("newer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestCheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub needs a user agent")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"tag_name": "v9.9.9"}`))
	}))
	defer srv.Close()
	oldURL := apiURL
	apiURL = srv.URL
	defer func() { apiURL = oldURL; latest = "" }()

	if err := check(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.RLock()
	got := latest
	mu.RUnlock()
	if got != "9.9.9" {
		t.Errorf("latest = %q, want 9.9.9 (without the v)", got)
	}

	latest = "1.0.0"
	status = http.StatusForbidden // e.g. GitHub's rate limit: keeps what it knew
	if err := check(context.Background()); err == nil {
		t.Error("a refused request is no error")
	}
	if latest != "1.0.0" {
		t.Errorf("latest = %q after a failed check, want it kept", latest)
	}
}
