// Package account contains the accounts users log in with.
package account

import (
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
)

var (
	ErrInvalidUsername = errors.New("username must be 3 to 32 characters: letters, digits, '.', '-' or '_'")
	ErrInvalidPassword = errors.New("password must be 8 to 72 bytes long")
	ErrInvalidEmail    = errors.New("enter a valid email address")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// Account is fixed to one player, which represents the account in games.
type Account struct {
	bun.BaseModel `bun:"table:accounts"`

	ID       int64  `bun:",pk,autoincrement" json:"id"`
	Username string `bun:",notnull,unique" json:"username"`
	// Email is optional (empty without one) and not confirmed: nothing is sent to it yet. It is unique among the accounts that have one.
	Email        string `bun:",notnull,default:''" json:"email,omitempty"`
	PasswordHash string `bun:",notnull" json:"-"`
	PlayerID     int64  `bun:",notnull,unique" json:"player_id"`
	// Language of the web interface (see i18n.Lang); empty: the browser's.
	Language  string    `bun:",notnull,default:''" json:"language"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	// Admin may change the settings of the server, e.g. the imprint. The first account
	// registered is the admin.
	Admin bool `bun:",notnull,default:false" json:"admin"`
	// Invisible hides from the account's friends whether it is online or playing: they see
	// it offline (incognito).
	Invisible bool `bun:",notnull,default:false" json:"invisible"`
	// LastSeenAt is when the account last made a request or had a page with live updates
	// open, for its friends (written at most once a minute); nil if never. Not in the API:
	// the friends only see whether it is online.
	LastSeenAt *time.Time `bun:",nullzero" json:"-"`
	// Disabled accounts can't log in; an admin disables and enables them. Admins can't be
	// disabled.
	Disabled bool `bun:",notnull,default:false" json:"-"`
}

// NormalizeUsername makes usernames case-insensitive.
func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

// NormalizeEmail trims the email and makes it lower case; it may be empty.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail checks that the normalized email is empty or a plain address like
// name@example.com.
func ValidateEmail(email string) error {
	if email == "" {
		return nil
	}
	a, err := mail.ParseAddress(email)
	_, domain, _ := strings.Cut(email, "@")
	if err != nil || a.Address != email || len(email) > 254 || !strings.Contains(domain, ".") {
		return ErrInvalidEmail
	}
	return nil
}

func ValidatePassword(password string) error {
	// bcrypt only supports up to 72 bytes.
	if len(password) < 8 || len(password) > 72 {
		return ErrInvalidPassword
	}
	return nil
}

// Session is a login. Only the SHA-256 hash of the token is stored,
// the token itself is only known to the client (in a cookie).
type Session struct {
	bun.BaseModel `bun:"table:sessions"`

	TokenHash string    `bun:",pk"`
	AccountID int64     `bun:",notnull"`
	ExpiresAt time.Time `bun:",notnull"`
}

type FriendshipStatus string

const (
	FriendshipPending  FriendshipStatus = "pending"
	FriendshipAccepted FriendshipStatus = "accepted"
)

// Friendship between two accounts. It starts as a request from the requester
// and counts once the addressee accepted. There is at most one per pair, in either direction.
type Friendship struct {
	bun.BaseModel `bun:"table:friendships"`

	RequesterID int64            `bun:",pk"`
	AddresseeID int64            `bun:",pk"`
	Status      FriendshipStatus `bun:",notnull"`
	CreatedAt   time.Time        `bun:",nullzero,notnull,default:current_timestamp"`
}

// Other returns the account on the other side of the friendship.
func (f Friendship) Other(accountID int64) int64 {
	if f.RequesterID == accountID {
		return f.AddresseeID
	}
	return f.RequesterID
}

// Controls reports whether the player is the account's: its own player or one of its
// guests. An account takes part in a game or tournament through the players it controls.
func (a Account) Controls(p player.Player) bool {
	return p.ID == a.PlayerID || (p.GuestOf != nil && *p.GuestOf == a.ID)
}
