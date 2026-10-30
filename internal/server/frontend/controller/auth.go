package controller

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// renderAuth shows the login or registration page, completing the model: whether
// registration is open and the key of the captcha widget, empty if registrations aren't
// checked.
func (c *Controller) renderAuth(w http.ResponseWriter, r *http.Request, status int, page func(view.AuthModel) templ.Component, m view.AuthModel) {
	open, err := c.svc.RegistrationOpen(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	m.Registration = open
	if m.CaptchaSiteKey, err = c.svc.CaptchaSiteKey(r.Context()); err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, page(m))
}

func (c *Controller) showLogin(w http.ResponseWriter, r *http.Request) {
	c.renderAuth(w, r, http.StatusOK, view.Login, view.AuthModel{})
}

func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	_, err := auth.Login(w, r, c.svc, username, r.FormValue("password"))
	if errors.Is(err, service.ErrUnauthenticated) {
		m := view.AuthModel{Username: username, Error: view.T(r.Context()).ErrLogin()}
		c.renderAuth(w, r, http.StatusUnauthorized, view.Login, m)
		return
	}
	if errors.Is(err, service.ErrTooManyLogins) {
		m := view.AuthModel{Username: username, Error: view.T(r.Context()).ErrTooManyLogins()}
		c.renderAuth(w, r, http.StatusTooManyRequests, view.Login, m)
		return
	}
	if errors.Is(err, service.ErrAccountDisabled) {
		m := view.AuthModel{Username: username, Error: view.T(r.Context()).ErrAccountDisabled()}
		c.renderAuth(w, r, http.StatusForbidden, view.Login, m)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Controller) showRegister(w http.ResponseWriter, r *http.Request) {
	c.renderAuth(w, r, http.StatusOK, view.Register, view.AuthModel{})
}

// register creates the account and logs it in right away.
func (c *Controller) register(w http.ResponseWriter, r *http.Request) {
	m := view.AuthModel{Username: r.FormValue("username"), Email: r.FormValue("email"), Name: r.FormValue("name")}
	password := r.FormValue("password")

	// Turnstile's widget adds its answer to the form under this name.
	err := c.svc.CheckCaptcha(r.Context(), r.FormValue("cf-turnstile-response"))
	if err == nil {
		_, err = c.svc.Register(r.Context(), m.Username, m.Email, m.Name, password)
	}
	t := view.T(r.Context())
	switch msg, ok := errorText(t, err); {
	case ok:
		m.Error = msg
	case errors.Is(err, service.ErrInvalid):
		m.Error = t.ErrRegisterInput()
	case err != nil:
		serverError(w, err)
		return
	}
	if m.Error != "" {
		c.renderAuth(w, r, http.StatusBadRequest, view.Register, m)
		return
	}

	if _, err := auth.Login(w, r, c.svc, m.Username, password); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Controller) logout(w http.ResponseWriter, r *http.Request) {
	if err := auth.Logout(w, r, c.svc); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
