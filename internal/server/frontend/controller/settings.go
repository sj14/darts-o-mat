package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func (c *Controller) showSettings(w http.ResponseWriter, r *http.Request) {
	c.renderSettings(w, r, http.StatusOK, "")
}

func (c *Controller) renderSettings(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	acc := auth.Account(r.Context())
	p, err := c.svc.Player(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Settings(acc.Username, acc.Email, p, errMsg))
}

// setColor stores the accent color of the account's player and shows the settings again.
func (c *Controller) setColor(w http.ResponseWriter, r *http.Request) {
	err := c.svc.SetColor(r.Context(), auth.Account(r.Context()), r.FormValue("color"))
	if err != nil && !errors.Is(err, service.ErrInvalid) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// setEmail changes the email of the account.
func (c *Controller) setEmail(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.SetEmail(r.Context(), auth.Account(r.Context()), r.FormValue("email"))
	c.respondSettings(w, r, err)
}

// deleteAccount deletes the account after its password was entered again, and logs out.
func (c *Controller) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if err := c.svc.DeleteAccount(r.Context(), auth.Account(r.Context()), r.FormValue("password")); err != nil {
		c.respondSettings(w, r, err)
		return
	}
	if err := auth.Logout(w, r, c.svc); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// respondSettings answers a change on the settings page: back to it, with the message of
// the error if it failed.
func (c *Controller) respondSettings(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	if msg, ok := errorText(view.T(r.Context()), err); ok {
		c.renderSettings(w, r, http.StatusBadRequest, msg)
		return
	}
	serverError(w, err)
}
