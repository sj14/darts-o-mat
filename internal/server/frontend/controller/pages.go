package controller

import (
	"errors"
	"log"
	"maps"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// withPages tells the footer which pages have a text.
func (c *Controller) withPages(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages, err := c.svc.Pages(r.Context())
		if err != nil {
			log.Printf("pages: %v", err)
		}
		next.ServeHTTP(w, r.WithContext(view.WithPages(r.Context(), pages)))
	})
}

// showPage shows the imprint or the privacy policy, 404 while it has no text. Search engines
// don't list it (noindex), so the operator's name and address aren't found through them.
func (c *Controller) showPage(name page.Name) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pages, err := c.svc.Pages(r.Context())
		if err != nil {
			serverError(w, err)
			return
		}
		text := pages[name]
		if text == "" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Robots-Tag", "noindex")
		render(w, r, http.StatusOK, view.Page(auth.Account(r.Context()).Username, name, text))
	}
}

// showAdmin shows the admin page; 404 for everyone but the admin, so it isn't revealed.
func (c *Controller) showAdmin(w http.ResponseWriter, r *http.Request) {
	c.renderAdmin(w, r, http.StatusOK, nil, "")
}

// renderAdmin shows the admin page with the stored texts, replaced by those in entered (a
// form that failed).
func (c *Controller) renderAdmin(w http.ResponseWriter, r *http.Request, status int, entered map[page.Name]string, errMsg string) {
	acc := auth.Account(r.Context())
	pages, err := c.svc.PagesToEdit(r.Context(), acc)
	if errors.Is(err, service.ErrForbidden) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	pages = maps.Clone(pages)
	maps.Copy(pages, entered)
	render(w, r, status, view.Admin(acc.Username, pages, r.URL.Query().Has("saved"), errMsg))
}

// setPage stores the text of a page.
func (c *Controller) setPage(w http.ResponseWriter, r *http.Request) {
	name, text := page.Name(chi.URLParam(r, "name")), r.FormValue("text")
	err := c.svc.SetPage(r.Context(), auth.Account(r.Context()), name, text)
	switch {
	case err == nil:
		http.Redirect(w, r, "/admin?saved", http.StatusSeeOther)
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrNotFound):
		http.NotFound(w, r)
	default:
		if msg, ok := errorText(view.T(r.Context()), err); ok {
			c.renderAdmin(w, r, http.StatusBadRequest, map[page.Name]string{name: text}, msg)
			return
		}
		serverError(w, err)
	}
}
