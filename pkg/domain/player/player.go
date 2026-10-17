package player

import "github.com/uptrace/bun"

// Player takes part in games. Every account has exactly one player;
// guest players have no account and belong to the account in GuestOf.
type Player struct {
	bun.BaseModel `bun:"table:players"`

	ID      int64  `bun:",pk,autoincrement" json:"id"`
	Name    string `bun:",notnull" json:"name"`
	GuestOf *int64 `json:"guest_of,omitempty"`
}

func (p Player) IsGuest() bool {
	return p.GuestOf != nil
}
