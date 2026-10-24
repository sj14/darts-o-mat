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

// GetPage returns the text of a page, empty while it has none.
func (store *Store) GetPage(ctx context.Context, name page.Name) (string, error) {
	var texts []string
	err := store.db.NewSelect().Model((*page.Page)(nil)).Column("text").Where("name = ?", name).Scan(ctx, &texts)
	if err != nil || len(texts) == 0 {
		return "", err
	}
	return texts[0], nil
}
