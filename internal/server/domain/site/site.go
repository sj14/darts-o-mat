// Package site has the settings of the server that the admin changes on the admin page.
package site

import "github.com/uptrace/bun"

// Settings are the admin's settings of the server, one row in the database.
type Settings struct {
	bun.BaseModel `bun:"table:site_settings"`

	// ID is always 1: there is one row.
	ID int64 `bun:",pk"`
	// Registration allows anyone to create an account. A new server is open, so the first
	// account, the admin, can register.
	Registration bool `bun:",notnull,default:true"`
}
