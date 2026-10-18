package player

import (
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// Colors are the accent colors a player can have, by name; the web interface knows how
// to show each of them.
var Colors = []string{"green", "red", "sky", "violet", "pink", "orange", "teal", "indigo", "fuchsia", "cyan"}

// Player takes part in games. Every account has exactly one player;
// guest players have no account and belong to the account in GuestOf.
type Player struct {
	bun.BaseModel `bun:"table:players"`

	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Name    string `bun:",notnull" json:"name"`
	GuestOf *int64 `json:"guest_of,omitempty"`
	// Color is the accent color the player chose (one of Colors); empty means the default
	// from the name, see Accent.
	Color string `json:"color,omitempty"`
	// DeletedAt is set when the player was deleted, as a guest or with their account: the
	// name is gone and the player belongs to nobody, but the games and darts stay, so the
	// statistics of the others don't change. It is shown as "deleted player".
	DeletedAt *time.Time `bun:",nullzero" json:"deleted_at,omitempty"`
}

// Deleted reports whether the player was deleted.
func (p Player) Deleted() bool {
	return p.DeletedAt != nil
}

// Accent returns the player's accent color: the chosen one, else one of Colors picked by
// the hash of the name, so it stays the same without storing anything.
func (p Player) Accent() string {
	if slices.Contains(Colors, p.Color) {
		return p.Color
	}
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(p.Name)))
	return Colors[h.Sum32()%uint32(len(Colors))]
}

func (p Player) IsGuest() bool {
	return p.GuestOf != nil
}
