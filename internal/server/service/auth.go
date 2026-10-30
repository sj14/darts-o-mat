package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sj14/darts-o-mat/internal/common/i18n"
	"github.com/sj14/darts-o-mat/internal/server/captcha"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

const SessionDuration = 30 * 24 * time.Hour

var (
	// ErrUnauthenticated means the login failed or the session is unknown or expired.
	ErrUnauthenticated    = errors.New("not logged in")
	ErrRegistrationClosed = errors.New("registration is disabled")
	// ErrAccountDisabled means an admin disabled the account: the password is right, but it
	// can't log in.
	ErrAccountDisabled = errors.New("the account is disabled")
	// ErrCaptcha means the check that the registration comes from a person failed.
	ErrCaptcha = fmt.Errorf("%w: the captcha check failed", ErrInvalid)
)

// CaptchaConfigured says whether the server has the keys of the captcha, so the admin can
// turn the check on and off.
func (s *Service) CaptchaConfigured() bool {
	return s.opts.Captcha != nil
}

// captcha returns the check registering needs: nil without keys or while the admin has
// turned it off.
func (s *Service) captcha(ctx context.Context) (captcha.Verifier, error) {
	if s.opts.Captcha == nil {
		return nil, nil
	}
	settings, err := s.store.SiteSettings(ctx)
	if err != nil || !settings.Captcha {
		return nil, err
	}
	return s.opts.Captcha, nil
}

// CaptchaSiteKey is the public key of the widget the registration page shows, empty if
// registrations aren't checked.
func (s *Service) CaptchaSiteKey(ctx context.Context) (string, error) {
	verifier, err := s.captcha(ctx)
	if verifier == nil {
		return "", err
	}
	return verifier.SiteKey(), nil
}

// CheckCaptcha checks the answer of the registration page's widget before Register:
// ErrCaptcha if it is wrong or missing, another error if the check couldn't be done. It
// does nothing if registrations aren't checked. An answer works once.
func (s *Service) CheckCaptcha(ctx context.Context, token string) error {
	verifier, err := s.captcha(ctx)
	if verifier == nil {
		return err
	}
	if err := verifier.Verify(ctx, token); errors.Is(err, captcha.ErrRejected) {
		return ErrCaptcha
	} else if err != nil {
		return err
	}
	return nil
}

// Register creates an account and its player. The player's name defaults to the username;
// the email may be empty.
func (s *Service) Register(ctx context.Context, username, email, name, password string) (account.Account, error) {
	switch open, err := s.RegistrationOpen(ctx); {
	case err != nil:
		return account.Account{}, err
	case !open:
		return account.Account{}, ErrRegistrationClosed
	}

	username = account.NormalizeUsername(username)
	if err := account.ValidateUsername(username); err != nil {
		return account.Account{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	email = account.NormalizeEmail(email)
	if err := account.ValidateEmail(email); err != nil {
		return account.Account{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := account.ValidatePassword(password); err != nil {
		return account.Account{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = username
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.opts.PasswordCost)
	if err != nil {
		return account.Account{}, err
	}

	a := account.Account{Username: username, Email: email, PasswordHash: string(hash)}
	err = s.store.CreateAccount(ctx, &a, &player.Player{Name: name})
	return a, err
}

// SetEmail changes the email of the account; empty removes it. It isn't confirmed, nothing
// is sent to it.
func (s *Service) SetEmail(ctx context.Context, acc account.Account, email string) (account.Account, error) {
	email = account.NormalizeEmail(email)
	if err := account.ValidateEmail(email); err != nil {
		return acc, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.SetEmail(ctx, acc.ID, email); err != nil {
		return acc, err
	}
	acc.Email = email
	return acc, nil
}

// ChangePassword sets a new password after checking the current one (ErrWrongPassword).
// The account's other sessions end, so whoever knew the old password is logged out; the
// session of token stays.
func (s *Service) ChangePassword(ctx context.Context, acc account.Account, current, password, token string) error {
	if _, err := s.checkPassword(ctx, acc, current); err != nil {
		return err
	}
	if err := account.ValidatePassword(password); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.opts.PasswordCost)
	if err != nil {
		return err
	}
	return s.store.SetPassword(ctx, acc.ID, string(hash), hashToken(token))
}

// checkPassword returns the stored account if the password is its own, else
// ErrWrongPassword; for changes that ask for the password again.
func (s *Service) checkPassword(ctx context.Context, acc account.Account, password string) (account.Account, error) {
	a, err := s.store.GetAccount(ctx, acc.ID)
	if err != nil {
		return a, err
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return a, ErrWrongPassword
	}
	return a, nil
}

// Login checks the credentials and starts a session. The returned token identifies the session.
// With a limit (Options.MaxLoginFailures) a username, or a network (WithNetwork), with too
// many failed logins lately is refused with ErrTooManyLogins, whether the username exists or
// not and whatever the password is.
func (s *Service) Login(ctx context.Context, username, password string) (string, account.Account, error) {
	username = account.NormalizeUsername(username)
	network := networkOf(ctx)
	now := s.now()
	if s.logins == nil {
		return s.login(ctx, username, password)
	}
	if blocked, err := s.loginBlocked(ctx, username, network, now); err != nil || blocked {
		if err == nil {
			err = ErrTooManyLogins
		}
		return "", account.Account{}, err
	}
	token, a, err := s.login(ctx, username, password)
	// Counting is done after the answer is known; if storing it fails, the answer stays.
	var countErr error
	switch {
	case errors.Is(err, ErrUnauthenticated):
		countErr = s.logins.fail(ctx, username, now)
		if network != "" {
			countErr = errors.Join(countErr, s.networks.fail(ctx, network, now))
		}
	case err == nil:
		countErr = s.logins.reset(ctx, username)
	}
	if countErr != nil {
		log.Printf("count the failed logins: %v", countErr)
	}
	return token, a, err
}

// loginBlocked reports whether the username, or the network if it is known, used up its
// failed logins.
func (s *Service) loginBlocked(ctx context.Context, username, network string, now time.Time) (bool, error) {
	blocked, err := s.logins.blocked(ctx, username, now)
	if err != nil || blocked || network == "" {
		return blocked, err
	}
	return s.networks.blocked(ctx, network, now)
}

func (s *Service) login(ctx context.Context, username, password string) (string, account.Account, error) {
	a, err := s.store.GetAccountByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(s.dummyHash, []byte(password))
		return "", account.Account{}, ErrUnauthenticated
	}
	if err != nil {
		return "", account.Account{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return "", account.Account{}, ErrUnauthenticated
	}
	// Only after the password, so it doesn't tell others which accounts are disabled.
	if a.Disabled {
		return "", account.Account{}, ErrAccountDisabled
	}

	token, err := newToken()
	if err != nil {
		return "", account.Account{}, err
	}

	now := s.now()
	session := account.Session{TokenHash: hashToken(token), AccountID: a.ID, ExpiresAt: now.Add(SessionDuration)}
	if err := s.store.CreateSession(ctx, &session, now); err != nil {
		return "", account.Account{}, err
	}
	return token, a, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.DeleteSession(ctx, hashToken(token))
}

// Authenticate returns the account of a session token.
func (s *Service) Authenticate(ctx context.Context, token string) (account.Account, error) {
	session, err := s.store.GetSession(ctx, hashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return account.Account{}, ErrUnauthenticated
	}
	if err != nil {
		return account.Account{}, err
	}

	a, err := s.store.GetAccount(ctx, session.AccountID)
	// Disabling ends the sessions; this catches one a login made at the same moment.
	if errors.Is(err, store.ErrNotFound) || a.Disabled {
		return account.Account{}, ErrUnauthenticated
	}
	if err == nil {
		s.seen(ctx, a)
	}
	return a, err
}

// newToken returns a random secret, e.g. for a session.
func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SetLanguage sets the language of the account's web interface; empty means the
// browser's.
func (s *Service) SetLanguage(ctx context.Context, acc account.Account, lang string) (account.Account, error) {
	if lang != "" {
		l, ok := i18n.Parse(lang)
		if !ok {
			return acc, fmt.Errorf("%w: unsupported language %q", ErrInvalid, lang)
		}
		lang = string(l)
	}
	acc.Language = lang
	return acc, s.store.SetLanguage(ctx, &acc)
}

// SetInvisible sets whether the account hides from its friends that it is online or
// playing (incognito): they see it offline.
func (s *Service) SetInvisible(ctx context.Context, acc account.Account, invisible bool) (account.Account, error) {
	acc.Invisible = invisible
	return acc, s.store.SetInvisible(ctx, &acc)
}
