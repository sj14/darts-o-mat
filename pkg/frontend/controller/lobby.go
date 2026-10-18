package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

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
	boardID, err := boardFromForm(r)
	if err == nil {
		_, err = c.svc.JoinGame(r.Context(), auth.Account(r.Context()), id, boardID)
	}
	if c.joinFailed(w, r, err, view.T(r.Context()).ErrNoSeat()) {
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
	boardID, err := boardFromForm(r)
	if err == nil {
		_, err = c.svc.JoinTournament(r.Context(), auth.Account(r.Context()), id, boardID)
	}
	if c.joinFailed(w, r, err, view.T(r.Context()).ErrNoSeat()) {
		return
	}
	http.Redirect(w, r, tournamentURL(id), http.StatusSeeOther)
}

// boardFromForm reads the board a player chose to throw on when joining a game or
// tournament or accepting an invitation; nil for none.
func boardFromForm(r *http.Request) (*int64, error) {
	s := r.FormValue("board_id")
	if s == "" {
		return nil, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid board id", service.ErrInvalid)
	}
	return &id, nil
}

// joinFailed answers a failed join or accept, if it failed: the start page with the
// message.
func (c *Controller) joinFailed(w http.ResponseWriter, r *http.Request, err error, msg string) bool {
	switch {
	case err == nil:
		return false
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrInvalid):
		c.renderHome(w, r, http.StatusConflict, "", msg)
	default:
		serverError(w, err)
	}
	return true
}
