package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showLogin(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, view.Login(view.AuthModel{Registration: c.svc.RegistrationOpen()}))
}

func (c *Controller) login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	_, err := auth.Login(w, r, c.svc, username, r.FormValue("password"))
	if errors.Is(err, service.ErrUnauthenticated) {
		m := view.AuthModel{Username: username, Error: view.T(r.Context()).ErrLogin(), Registration: c.svc.RegistrationOpen()}
		render(w, r, http.StatusUnauthorized, view.Login(m))
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (c *Controller) showRegister(w http.ResponseWriter, r *http.Request) {
	render(w, r, http.StatusOK, view.Register(view.AuthModel{Registration: c.svc.RegistrationOpen()}))
}

// register creates the account and logs it in right away.
func (c *Controller) register(w http.ResponseWriter, r *http.Request) {
	m := view.AuthModel{Username: r.FormValue("username"), Email: r.FormValue("email"), Name: r.FormValue("name"), Registration: c.svc.RegistrationOpen()}
	password := r.FormValue("password")

	_, err := c.svc.Register(r.Context(), m.Username, m.Email, m.Name, password)
	switch {
	case errors.Is(err, service.ErrUsernameTaken):
		m.Error = view.T(r.Context()).ErrUsernameTaken()
	case errors.Is(err, service.ErrEmailTaken):
		m.Error = view.T(r.Context()).ErrEmailTaken()
	case errors.Is(err, account.ErrInvalidEmail):
		m.Error = view.T(r.Context()).ErrInvalidEmail()
	case errors.Is(err, service.ErrInvalid):
		m.Error = view.T(r.Context()).ErrRegisterInput()
	case errors.Is(err, service.ErrRegistrationClosed):
		m.Error = view.T(r.Context()).RegistrationClosed()
	case err != nil:
		serverError(w, err)
		return
	}
	if m.Error != "" {
		render(w, r, http.StatusBadRequest, view.Register(m))
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
