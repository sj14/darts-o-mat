package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// The registration page shows Cloudflare's widget only with a site key.
func TestRegisterShowsTheCaptchaOnlyWithAKey(t *testing.T) {
	t.Parallel()
	render := func(m AuthModel) string {
		var b bytes.Buffer
		m.Registration = true
		if err := Register(m).Render(context.Background(), &b); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	if page := render(AuthModel{}); strings.Contains(page, "cloudflare") || strings.Contains(page, "cf-turnstile") {
		t.Error("the registration page without a key contacts Cloudflare")
	}
	page := render(AuthModel{CaptchaSiteKey: "0x4AAAsite"})
	if !strings.Contains(page, `data-sitekey="0x4AAAsite"`) || !strings.Contains(page, "challenges.cloudflare.com/turnstile/v0/api.js") {
		t.Error("the registration page with a key has no widget")
	}
}
