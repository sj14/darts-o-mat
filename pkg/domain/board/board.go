package board

import "time"

// Board is a physical dartboard of an account.
type Board struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	AccountID int64     `bun:",notnull" json:"-"`
	Name      string    `bun:",notnull" json:"name"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// Token lets the board's cameras (darts-o-cam) report darts without a user login.
// Only the SHA-256 hash of the token is stored; a board has at most one.
type Token struct {
	BoardID   int64     `bun:",pk" json:"board_id"`
	TokenHash string    `bun:",notnull,unique" json:"-"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}
