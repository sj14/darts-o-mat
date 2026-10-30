package captcha

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTurnstile(t *testing.T) {
	t.Parallel()
	var got struct{ secret, response string }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		got.secret, got.response = r.PostForm.Get("secret"), r.PostForm.Get("response")
		if got.secret != "secret" {
			_, _ = w.Write([]byte(`{"success": false, "error-codes": ["invalid-input-secret"]}`))
			return
		}
		if got.response == "good" {
			_, _ = w.Write([]byte(`{"success": true, "error-codes": []}`))
			return
		}
		_, _ = w.Write([]byte(`{"success": false, "error-codes": ["invalid-input-response"]}`))
	}))
	defer srv.Close()
	v := NewTurnstile("site", "secret")
	v.URL = srv.URL
	ctx := context.Background()

	if v.SiteKey() != "site" {
		t.Errorf("site key %q", v.SiteKey())
	}
	if err := v.Verify(ctx, "good"); err != nil || got.secret != "secret" || got.response != "good" {
		t.Errorf("good token: %v, sent %+v", err, got)
	}
	if err := v.Verify(ctx, "bad"); !errors.Is(err, ErrRejected) {
		t.Errorf("bad token: got %v, want ErrRejected", err)
	}
	got.response = "untouched"
	if err := v.Verify(ctx, ""); !errors.Is(err, ErrRejected) || got.response != "untouched" {
		t.Errorf("no token: got %v (asked Cloudflare: %v), want ErrRejected without asking", err, got.response != "untouched")
	}

	// A wrong secret key is the server's fault, not a rejection of the person.
	wrong := NewTurnstile("site", "wrong")
	wrong.URL = srv.URL
	if err := wrong.Verify(ctx, "good"); err == nil || errors.Is(err, ErrRejected) {
		t.Errorf("wrong secret key: got %v, want another error", err)
	}

	// Cloudflare not reachable or answering badly isn't a rejection of the person.
	srv.Close()
	if err := v.Verify(ctx, "good"); err == nil || errors.Is(err, ErrRejected) {
		t.Errorf("unreachable: got %v, want another error", err)
	}
}
