package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/captcha"
)

// fakeCaptcha accepts the token "human", rejects others and fails for "down".
type fakeCaptcha struct{}

func (fakeCaptcha) SiteKey() string { return "site" }

func (fakeCaptcha) Verify(_ context.Context, token string) error {
	switch token {
	case "human":
		return nil
	case "down":
		return errors.New("cloudflare is down")
	}
	return captcha.ErrRejected
}

func TestCaptcha(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)

	// Without keys nothing is checked, whatever the token.
	if key, err := s.CaptchaSiteKey(ctx); key != "" || err != nil || s.CheckCaptcha(ctx, "") != nil {
		t.Error("a server without a captcha checks or shows one")
	}

	s.opts.Captcha = fakeCaptcha{}
	if key, err := s.CaptchaSiteKey(ctx); key != "site" || err != nil {
		t.Errorf("site key %q, %v, want the verifier's", key, err)
	}
	if err := s.CheckCaptcha(ctx, "human"); err != nil {
		t.Errorf("a person: %v", err)
	}
	for _, token := range []string{"", "robot"} {
		if err := s.CheckCaptcha(ctx, token); !errors.Is(err, ErrCaptcha) || !errors.Is(err, ErrInvalid) {
			t.Errorf("token %q: got %v, want ErrCaptcha (an ErrInvalid)", token, err)
		}
	}
	if err := s.CheckCaptcha(ctx, "down"); err == nil || errors.Is(err, ErrCaptcha) {
		t.Errorf("check not possible: got %v, want another error", err)
	}
}

// Only the admin turns the captcha off and on again; while it is off, the page shows no
// widget and registering isn't checked.
func TestSetCaptcha(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	s.opts.Captcha = fakeCaptcha{}
	admin, ben := register(t, s, "anna"), register(t, s, "ben")

	if err := s.SetCaptcha(ctx, ben, false); !errors.Is(err, ErrForbidden) {
		t.Errorf("turning it off as ben: got %v, want ErrForbidden", err)
	}
	if err := s.SetCaptcha(ctx, admin, false); err != nil {
		t.Fatal(err)
	}
	if key, err := s.CaptchaSiteKey(ctx); key != "" || err != nil {
		t.Errorf("site key while off: %q, %v, want none", key, err)
	}
	if err := s.CheckCaptcha(ctx, "robot"); err != nil {
		t.Errorf("checked while off: %v", err)
	}

	if err := s.SetCaptcha(ctx, admin, true); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckCaptcha(ctx, "robot"); !errors.Is(err, ErrCaptcha) {
		t.Errorf("on again: got %v, want ErrCaptcha", err)
	}
}
