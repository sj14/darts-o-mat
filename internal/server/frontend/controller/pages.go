package controller

import (
	"errors"
	"maps"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// showPage shows the imprint or the privacy policy; while it has no text, the page says so
// with 404. Search engines don't list it (noindex), so the operator's name and address
// aren't found through them.
func (c *Controller) showPage(name page.Name) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		text, err := c.svc.Page(r.Context(), name)
		if err != nil {
			serverError(w, err)
			return
		}
		status := http.StatusOK
		if text == "" {
			status = http.StatusNotFound
		}
		w.Header().Set("X-Robots-Tag", "noindex")
		render(w, r, status, view.Page(auth.Account(r.Context()).Username, name, text))
	}
}

// showAdmin shows the admin page; 404 for everyone but an admin, so it isn't revealed.
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
	open, err := c.svc.RegistrationOpen(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	admins, err := c.svc.Admins(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	maps.Copy(pages, entered)
	m := view.AdminModel{
		Admins:       admins,
		Pages:        pages,
		Registration: open,
		Captcha:      c.svc.CaptchaSiteKey() != "",
		Saved:        r.URL.Query().Has("saved"),
		Error:        errMsg,
	}
	render(w, r, status, view.Admin(acc.Username, m))
}

// setRegistration opens or closes the registration.
func (c *Controller) setRegistration(w http.ResponseWriter, r *http.Request) {
	err := c.svc.SetRegistration(r.Context(), auth.Account(r.Context()), r.FormValue("open") == "true")
	switch {
	case err == nil:
		http.Redirect(w, r, "/admin?saved", http.StatusSeeOther)
	case errors.Is(err, service.ErrForbidden):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

// setAdmin gives an account the admin role or takes it away. Whoever gave up their own role
// goes to the settings, since the admin page is gone for them.
func (c *Controller) setAdmin(w http.ResponseWriter, r *http.Request) {
	acc := auth.Account(r.Context())
	username, admin := r.FormValue("username"), r.FormValue("admin") == "true"
	err := c.svc.SetAdmin(r.Context(), acc, username, admin)
	switch {
	case err == nil && !admin && account.NormalizeUsername(username) == acc.Username:
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	case err == nil:
		http.Redirect(w, r, "/admin?saved", http.StatusSeeOther)
	case errors.Is(err, service.ErrForbidden):
		http.NotFound(w, r)
	default:
		if msg, ok := errorText(view.T(r.Context()), err); ok {
			c.renderAdmin(w, r, http.StatusBadRequest, nil, msg)
			return
		}
		serverError(w, err)
	}
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
