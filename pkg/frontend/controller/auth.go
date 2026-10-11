package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
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
		m := view.AuthModel{Username: username, Error: "Benutzername oder Passwort ist falsch.", Registration: c.svc.RegistrationOpen()}
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
	m := view.AuthModel{Username: r.FormValue("username"), Name: r.FormValue("name"), Registration: c.svc.RegistrationOpen()}
	password := r.FormValue("password")

	_, err := c.svc.Register(r.Context(), m.Username, m.Name, password)
	switch {
	case errors.Is(err, service.ErrUsernameTaken):
		m.Error = "Dieser Benutzername ist schon vergeben."
	case errors.Is(err, service.ErrInvalid):
		m.Error = "Bitte Benutzername (3 bis 32 Zeichen: Buchstaben, Ziffern, . - _) und Passwort (mindestens 8 Zeichen) prüfen."
	case errors.Is(err, service.ErrRegistrationClosed):
		m.Error = "Die Registrierung ist abgeschaltet."
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
