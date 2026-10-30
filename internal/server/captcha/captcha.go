// Package captcha checks that a registration comes from a person, with Cloudflare
// Turnstile. It is optional: a server without keys doesn't check anything (see the server's
// documentation).
package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// ErrRejected means the check failed: the token is missing, wrong, expired or was used
// before. Other errors are about reaching Cloudflare or a wrong secret key.
var ErrRejected = errors.New("captcha rejected")

// Verifier checks the answer of the widget a page showed.
type Verifier interface {
	// SiteKey is the public key of the widget the page has to show.
	SiteKey() string
	// Verify returns ErrRejected if the token isn't valid.
	Verify(ctx context.Context, token string) error
}

// verifyURL is where Cloudflare checks tokens.
const verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// Turnstile checks tokens with Cloudflare Turnstile. The site key is public (in the page),
// the secret key stays on the server.
type Turnstile struct {
	Site, Secret string
	// URL replaces Cloudflare's address in tests.
	URL    string
	Client *http.Client
}

// NewTurnstile returns a Verifier for the keys of a Turnstile widget.
func NewTurnstile(siteKey, secretKey string) *Turnstile {
	return &Turnstile{Site: siteKey, Secret: secretKey, URL: verifyURL, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (t *Turnstile) SiteKey() string { return t.Site }

// Verify asks Cloudflare whether the token is valid. A token works once.
func (t *Turnstile) Verify(ctx context.Context, token string) error {
	if token == "" {
		return ErrRejected
	}
	form := url.Values{"secret": {t.Secret}, "response": {token}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Success bool     `json:"success"`
		Errors  []string `json:"error-codes"`
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("turnstile: status %d", res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&out); err != nil {
		return fmt.Errorf("turnstile: %w", err)
	}
	if !out.Success {
		if slices.ContainsFunc(out.Errors, isSecretError) {
			return fmt.Errorf("turnstile: check the secret key: %v", out.Errors)
		}
		return fmt.Errorf("%w: %v", ErrRejected, out.Errors)
	}
	return nil
}

// isSecretError reports whether Cloudflare's error code is about the server's secret key
// rather than the visitor's answer: the server is set up wrong, which belongs in the log.
func isSecretError(code string) bool {
	return code == "missing-input-secret" || code == "invalid-input-secret"
}
