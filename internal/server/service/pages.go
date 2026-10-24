package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

// Pages returns the texts of the pages that have one, by name; the footer links those. They
// are read from the database once and then kept in memory, like the live updates (one
// process). The map must not be changed.
func (s *Service) Pages(ctx context.Context) (map[page.Name]string, error) {
	s.pagesMu.Lock()
	defer s.pagesMu.Unlock()
	if s.pages != nil {
		return s.pages, nil
	}
	pages, err := s.store.ListPages(ctx)
	if err != nil {
		return nil, err
	}
	s.pages = map[page.Name]string{}
	for _, p := range pages {
		s.pages[p.Name] = p.Text
	}
	return s.pages, nil
}

// PagesToEdit returns the texts of the pages for the admin page; ErrForbidden for everyone
// but an admin.
func (s *Service) PagesToEdit(ctx context.Context, acc account.Account) (map[page.Name]string, error) {
	if err := checkAdmin(acc); err != nil {
		return nil, err
	}
	return s.Pages(ctx)
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
	if _, err := s.Pages(ctx); err != nil {
		return err
	}

	s.pagesMu.Lock()
	defer s.pagesMu.Unlock()
	if err := s.store.SetPage(ctx, p); err != nil {
		return err
	}
	// A new map, so whoever still reads the old one isn't disturbed.
	pages := maps.Clone(s.pages)
	if p.Text == "" {
		delete(pages, p.Name)
	} else {
		pages[p.Name] = p.Text
	}
	s.pages = pages
	return nil
}

// checkAdmin is the one check that an account may change the settings of the server.
func checkAdmin(acc account.Account) error {
	if !acc.Admin {
		return ErrForbidden
	}
	return nil
}
