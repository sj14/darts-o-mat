// Package account contains the accounts users log in with.
package account

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

var (
	ErrInvalidUsername = errors.New("username must be 3 to 32 characters: letters, digits, '.', '-' or '_'")
	ErrInvalidPassword = errors.New("password must be 8 to 72 bytes long")
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

// Account is fixed to one player, which represents the account in games.
type Account struct {
	bun.BaseModel `bun:"table:accounts"`

	ID           int64  `bun:",pk,autoincrement" json:"id"`
	Username     string `bun:",notnull,unique" json:"username"`
	PasswordHash string `bun:",notnull" json:"-"`
	PlayerID     int64  `bun:",notnull,unique" json:"player_id"`
	// Language of the web interface (see i18n.Lang); empty: the browser's.
	Language  string    `bun:",notnull,default:''" json:"language"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
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
