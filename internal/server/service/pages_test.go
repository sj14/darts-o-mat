package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

// The first account is the admin and the only one who may write the pages; an empty text
// takes a page down.
func TestPages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	admin := register(t, s, "anna")
	ben := register(t, s, "ben")
	if !admin.Admin || ben.Admin {
		t.Fatalf("admin: anna %v, ben %v; want only anna", admin.Admin, ben.Admin)
	}

	if err := s.SetPage(ctx, ben, page.Imprint, "Ben"); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben sets the imprint: %v, want ErrForbidden", err)
	}
	if _, err := s.PagesToEdit(ctx, ben); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben opens the admin page: %v, want ErrForbidden", err)
	}
	if err := s.SetPage(ctx, admin, "terms", "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown page: %v, want ErrNotFound", err)
	}
	if err := s.SetPage(ctx, admin, page.Imprint, strings.Repeat("x", page.MaxLen+1)); !errors.Is(err, ErrInvalid) {
		t.Errorf("too long: %v, want ErrInvalid", err)
	}

	if err := s.SetPage(ctx, admin, page.Imprint, " Anna\r\nStreet 1 \n"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPage(ctx, admin, page.Privacy, "Nothing"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPage(ctx, admin, page.Privacy, ""); err != nil {
		t.Fatal(err)
	}

	// A new service reads them from the database.
	for _, svc := range []*Service{s, New(s.store, s.opts)} {
		pages, err := svc.Pages(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 || pages[page.Imprint] != "Anna\nStreet 1" {
			t.Errorf("pages = %q, want only the imprint", pages)
		}
	}
}
