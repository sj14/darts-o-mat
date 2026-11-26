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
var Colors = []string{"green", "red", "orange", "sky", "violet", "pink"}

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
	// Bot marks a guest the server throws for. Bots don't show in the account's lists of
	// guests: a free one is taken when a bot is added to a game, another one made only if
	// all are seated. How well it throws belongs to its seat (game.Participant.BotSpreadUM).
	Bot bool `bun:",notnull,default:false" json:"bot,omitempty"`
	// DeletedAt is set when the player was deleted, as a guest or with their account: the
	// name is gone and the player belongs to nobody, but the games and darts stay, so the
	// statistics of the others don't change. It is shown as "deleted player".
	DeletedAt *time.Time `bun:",nullzero" json:"deleted_at,omitempty"`
}

// Deleted reports whether the player was deleted.
func (p Player) Deleted() bool {
	return p.DeletedAt != nil
}

// ChosenColor returns the color the player chose, or empty when they chose none or one
// that is no longer in Colors; so Colors can change without touching the stored choices.
func (p Player) ChosenColor() string {
	if slices.Contains(Colors, p.Color) {
		return p.Color
	}
	return ""
}

// Accent returns the player's accent color: the chosen one, else one of Colors picked by
// the hash of the name, so it stays the same without storing anything.
func (p Player) Accent() string {
	if c := p.ChosenColor(); c != "" {
		return c
	}
	h := fnv.New32a()
	h.Write([]byte(strings.ToLower(p.Name)))
	return Colors[h.Sum32()%uint32(len(Colors))]
}

func (p Player) IsGuest() bool {
	return p.GuestOf != nil
}
