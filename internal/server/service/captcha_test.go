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
	ctx := context.Background()
	s := newService(t)

	// Without keys nothing is checked, whatever the token.
	if s.CaptchaSiteKey() != "" || s.CheckCaptcha(ctx, "") != nil {
		t.Error("a server without a captcha checks or shows one")
	}

	s.opts.Captcha = fakeCaptcha{}
	if s.CaptchaSiteKey() != "site" {
		t.Errorf("site key %q, want the verifier's", s.CaptchaSiteKey())
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
