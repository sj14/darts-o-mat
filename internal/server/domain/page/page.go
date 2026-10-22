// Package page has the texts the admin writes for pages of the server: the imprint and the
// privacy policy.
package page

import (
	"errors"
	"slices"
	"unicode/utf8"

	"github.com/uptrace/bun"
)

// MaxLen is the longest text of a page, in characters.
const MaxLen = 50000

var (
	ErrUnknown = errors.New("unknown page")
	ErrTooLong = errors.New("the text is too long")
)

// Name says which page a text is for.
type Name string

const (
	Imprint Name = "imprint"
	Privacy Name = "privacy"
)

// Names are the pages, in the order the footer and the admin page show them.
var Names = []Name{Imprint, Privacy}

// Page is the text of a page, plain text. A page without text isn't shown.
type Page struct {
	bun.BaseModel `bun:"table:pages"`

	Name Name   `bun:",pk"`
	Text string `bun:",notnull"`
}

// Validate checks that the page is known and its text not too long.
func (p Page) Validate() error {
	if !slices.Contains(Names, p.Name) {
		return ErrUnknown
	}
	if utf8.RuneCountInString(p.Text) > MaxLen {
		return ErrTooLong
	}
	return nil
}
