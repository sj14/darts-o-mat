package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
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
	switch _, err := c.svc.SetEmail(r.Context(), auth.Account(r.Context()), r.FormValue("email")); {
	case err == nil:
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	case errors.Is(err, service.ErrEmailTaken):
		c.renderSettings(w, r, http.StatusConflict, view.T(r.Context()).ErrEmailTaken())
	case errors.Is(err, account.ErrInvalidEmail):
		c.renderSettings(w, r, http.StatusBadRequest, view.T(r.Context()).ErrInvalidEmail())
	default:
		serverError(w, err)
	}
}

// deleteAccount deletes the account after its password was entered again, and logs out.
func (c *Controller) deleteAccount(w http.ResponseWriter, r *http.Request) {
	switch err := c.svc.DeleteAccount(r.Context(), auth.Account(r.Context()), r.FormValue("password")); {
	case err == nil:
		if err := auth.Logout(w, r, c.svc); err != nil {
			serverError(w, err)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case errors.Is(err, service.ErrWrongPassword):
		c.renderSettings(w, r, http.StatusForbidden, view.T(r.Context()).ErrWrongPassword())
	default:
		serverError(w, err)
	}
}
