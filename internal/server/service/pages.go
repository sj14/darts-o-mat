package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

// Page returns the text of a page, empty while it has none. It is read from the database
// only when the page is opened, so every replica shows what an admin saved.
func (s *Service) Page(ctx context.Context, name page.Name) (string, error) {
	return s.store.GetPage(ctx, name)
}

// PagesToEdit returns the texts of the pages that have one, by name, for the admin page;
// ErrForbidden for everyone but an admin.
func (s *Service) PagesToEdit(ctx context.Context, acc account.Account) (map[page.Name]string, error) {
	if err := checkAdmin(acc); err != nil {
		return nil, err
	}
	pages, err := s.store.ListPages(ctx)
	if err != nil {
		return nil, err
	}
	texts := map[page.Name]string{}
	for _, p := range pages {
		texts[p.Name] = p.Text
	}
	return texts, nil
}

// SetPage changes the text of a page; an empty text takes the page down. Only an admin may.
func (s *Service) SetPage(ctx context.Context, acc account.Account, name page.Name, text string) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	p := page.Page{Name: name, Text: strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))}
	switch err := p.Validate(); {
	case errors.Is(err, page.ErrUnknown):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return s.store.SetPage(ctx, p)
}

// checkAdmin is the one check that an account may change the settings of the server.
func checkAdmin(acc account.Account) error {
	if !acc.Admin {
		return ErrForbidden
	}
	return nil
}
