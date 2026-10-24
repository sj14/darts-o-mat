package view

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/page"
)

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

// pageMissingOf says that a page has no text yet.
func pageMissingOf(ctx context.Context, name page.Name) string {
	if name == page.Privacy {
		return T(ctx).NoPrivacy()
	}
	return T(ctx).NoImprint()
}
