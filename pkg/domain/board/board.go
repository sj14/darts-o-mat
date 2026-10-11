package board

import (
	"net/url"
	"time"
)

type Server struct {
	Host  url.URL
	Token string
}

// Board is a physical dartboard of an account.
type Board struct {
	ID        int64     `bun:",pk,autoincrement" json:"id"`
	AccountID int64     `bun:",notnull" json:"-"`
	Name      string    `bun:",notnull" json:"name"`
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
}

func Status() (string, error) {
	return "", nil
}

func Register() error {
	return nil
}

func Release() error {
	return nil
}
