package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/i18n"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

const SessionDuration = 30 * 24 * time.Hour

var (
	// ErrUnauthenticated means the login failed or the session is unknown or expired.
	ErrUnauthenticated    = errors.New("not logged in")
	ErrRegistrationClosed = errors.New("registration is disabled")
	ErrUsernameTaken      = store.ErrUsernameTaken
	ErrEmailTaken         = store.ErrEmailTaken
)

// dummyHash is compared against when a username doesn't exist,
// so a failed login takes the same time whether the username exists or not.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("darts-o-mat"), bcrypt.DefaultCost)

// Register creates an account and its player. The player's name defaults to the username;
// the email may be empty.
func (s *Service) Register(ctx context.Context, username, email, name, password string) (account.Account, error) {
	if !s.opts.Registration {
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

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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

// Login checks the credentials and starts a session. The returned token identifies the session.
func (s *Service) Login(ctx context.Context, username, password string) (string, account.Account, error) {
	a, err := s.store.GetAccountByUsername(ctx, account.NormalizeUsername(username))
	if errors.Is(err, store.ErrNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", account.Account{}, ErrUnauthenticated
	}
	if err != nil {
		return "", account.Account{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return "", account.Account{}, ErrUnauthenticated
	}

	token, err := newToken()
	if err != nil {
		return "", account.Account{}, err
	}

	session := account.Session{TokenHash: hashToken(token), AccountID: a.ID, ExpiresAt: time.Now().Add(SessionDuration)}
	if err := s.store.CreateSession(ctx, &session); err != nil {
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
	if errors.Is(err, store.ErrNotFound) {
		return account.Account{}, ErrUnauthenticated
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
