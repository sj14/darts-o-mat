package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/site"
)

// SiteSettings returns the admin's settings of the server.
func (store *Store) SiteSettings(ctx context.Context) (site.Settings, error) {
	var s site.Settings
	err := store.db.NewSelect().Model(&s).Where("id = 1").Scan(ctx)
	return s, err
}

// SetSiteSettings stores the admin's settings of the server.
func (store *Store) SetSiteSettings(ctx context.Context, s site.Settings) error {
	s.ID = 1
	_, err := store.db.NewUpdate().Model(&s).WherePK().Exec(ctx)
	return err
}
