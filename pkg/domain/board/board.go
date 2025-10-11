package board

import (
	"net/url"
)

type Server struct {
	Host  url.URL
	Token string
}

type Board struct {
	// gorm.Model
	ID uint64
	// Servers []Server
	// Registered string // GameID
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
