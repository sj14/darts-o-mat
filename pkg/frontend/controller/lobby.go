package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

// joinGame takes a seat in a game open for strangers and goes to it.
func (c *Controller) joinGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	_, err := c.svc.JoinGame(r.Context(), auth.Account(r.Context()), id)
	if c.joinFailed(w, r, err) {
		return
	}
	http.Redirect(w, r, gameURL(id), http.StatusSeeOther)
}

// joinTournament takes a seat in a tournament open for strangers and goes to it.
func (c *Controller) joinTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	_, err := c.svc.JoinTournament(r.Context(), auth.Account(r.Context()), id)
	if c.joinFailed(w, r, err) {
		return
	}
	http.Redirect(w, r, tournamentURL(id), http.StatusSeeOther)
}

// joinFailed answers a failed join, if it failed: the start page with the reason.
func (c *Controller) joinFailed(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return false
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrInvalid):
		c.renderHome(w, r, http.StatusConflict, "", view.T(r.Context()).ErrNoSeat())
	default:
		serverError(w, err)
	}
	return true
}
