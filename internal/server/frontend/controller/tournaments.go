package controller

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/common/sse"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func tournamentURL(id int64) string {
	return "/tournaments/" + strconv.FormatInt(id, 10)
}

func (c *Controller) showTournament(w http.ResponseWriter, r *http.Request) {
	c.renderTournament(w, r, http.StatusOK, view.Alert{})
}

func (c *Controller) renderTournament(w http.ResponseWriter, r *http.Request, status int, failed view.Alert) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	acc := auth.Account(r.Context())
	v, err := c.svc.Tournament(r.Context(), acc, id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Tournament(acc.Username, v, failed))
}

// tournamentEvents streams the tournament to its page whenever it or one of its games
// changes, like gameEvents.
func (c *Controller) tournamentEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	acc := auth.Account(r.Context())
	if _, err := c.svc.Tournament(r.Context(), acc, id); isNotFound(err) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	changes, stop := c.svc.SubscribeTournament(id)
	defer stop()
	sse.Stream(w, r, "tournament", changes, func() (string, error) {
		v, err := c.svc.Tournament(r.Context(), acc, id)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := view.TournamentBody(v, view.Alert{}).Render(r.Context(), &buf); err != nil {
			log.Printf("render tournament: %v", err)
			return "", err
		}
		return buf.String(), nil
	})
}

// startMatch starts the game of a match and goes to it.
func (c *Controller) startMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	match, err := strconv.Atoi(chi.URLParam(r, "match"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := c.svc.StartMatch(r.Context(), auth.Account(r.Context()), id, match)
	_, busy := errors.AsType[service.PlayerBusyError](err)
	switch {
	case err == nil:
		http.Redirect(w, r, gameURL(v.Game.ID), http.StatusSeeOther)
	case isNotFound(err):
		http.NotFound(w, r)
	case busy:
		msg, _ := errorText(view.T(r.Context()), err)
		c.renderTournament(w, r, http.StatusConflict, alertOf(err, msg))
	case errors.Is(err, service.ErrInvalid), errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrNotReady), errors.Is(err, service.ErrAborted):
		c.renderTournament(w, r, http.StatusConflict, view.Alert{Text: view.T(r.Context()).ErrMatchNotStartable()})
	default:
		serverError(w, err)
	}
}

// leaveTournament lets the account leave the tournament with all its players: in the
// lobby they are removed, once it is running it ends for everyone.
func (c *Controller) leaveTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.LeaveTournament(r.Context(), auth.Account(r.Context()), id)
	switch {
	case err == nil && v.Tournament.ID == 0: // left the lobby: nothing to show anymore
		http.Redirect(w, r, "/", http.StatusSeeOther)
	case err == nil, errors.Is(err, service.ErrAborted):
		http.Redirect(w, r, tournamentURL(id), http.StatusSeeOther)
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrInvalid):
		c.renderTournament(w, r, http.StatusConflict, view.Alert{Text: view.T(r.Context()).ErrInput()})
	default:
		serverError(w, err)
	}
}
