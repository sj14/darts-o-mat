package view

import (
	"bytes"
	"context"
	"html"
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

// With the registration closed the landing page still shows the register button, greyed out,
// and the landing and login pages say why.
func TestClosedRegistrationIsShown(t *testing.T) {
	t.Parallel()
	closed := T(context.Background()).RegistrationClosed()
	var landing, login bytes.Buffer
	if err := Landing(false, "").Render(context.Background(), &landing); err != nil {
		t.Fatal(err)
	}
	if err := Login(AuthModel{}).Render(context.Background(), &login); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(landing.String(), "disabled>"+T(context.Background()).Register()) || strings.Contains(landing.String(), `href="/register"`) {
		t.Error("the landing page doesn't grey out the register button")
	}
	for name, page := range map[string]string{"landing": landing.String(), "login": login.String()} {
		if !strings.Contains(page, html.EscapeString(closed)) {
			t.Errorf("the %s page doesn't say that the registration is closed", name)
		}
	}
}
