package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

// ListPages returns the pages that have a text.
func (store *Store) ListPages(ctx context.Context) ([]page.Page, error) {
	var pages []page.Page
	err := store.db.NewSelect().Model(&pages).Where("text <> ''").Scan(ctx)
	return pages, err
}

// SetPage stores the text of a page, replacing the one before.
func (store *Store) SetPage(ctx context.Context, p page.Page) error {
	_, err := store.db.NewInsert().Model(&p).On("CONFLICT (name) DO UPDATE").Set("text = EXCLUDED.text").Exec(ctx)
	return err
}
