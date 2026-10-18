package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showSettings(w http.ResponseWriter, r *http.Request) {
	acc := auth.Account(r.Context())
	p, err := c.svc.Player(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Settings(acc.Username, p))
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
