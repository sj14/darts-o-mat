package view

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

type pagesKey struct{}

// WithPages tells the footer which pages have a text (service.Pages).
func WithPages(ctx context.Context, pages map[page.Name]string) context.Context {
	return context.WithValue(ctx, pagesKey{}, pages)
}

// footerPages are the pages the footer links: those with a text, in the order of page.Names.
func footerPages(ctx context.Context) []page.Name {
	pages, _ := ctx.Value(pagesKey{}).(map[page.Name]string)
	var names []page.Name
	for _, name := range page.Names {
		if pages[name] != "" {
			names = append(names, name)
		}
	}
	return names
}

// PagePath is the address of a page: /imprint, /privacy.
func PagePath(name page.Name) string {
	return "/" + string(name)
}

// pageTitleOf is the heading of a page in the language of the request.
func pageTitleOf(ctx context.Context, name page.Name) string {
	switch name {
	case page.Imprint:
		return T(ctx).Imprint()
	case page.Privacy:
		return T(ctx).Privacy()
	}
	return string(name)
}
