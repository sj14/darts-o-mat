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
	c.renderTournament(w, r, http.StatusOK, "")
}

func (c *Controller) renderTournament(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
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
	render(w, r, status, view.Tournament(acc.Username, v, errMsg))
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
		if err := view.TournamentBody(v, "").Render(r.Context(), &buf); err != nil {
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
	switch {
	case err == nil:
		http.Redirect(w, r, gameURL(v.Game.ID), http.StatusSeeOther)
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrInvalid), errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrNotReady), errors.Is(err, service.ErrAborted):
		c.renderTournament(w, r, http.StatusConflict, view.T(r.Context()).ErrMatchNotStartable())
	default:
		serverError(w, err)
	}
}

func (c *Controller) acceptTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, err := boardFromForm(r)
	if err == nil {
		_, err = c.svc.AcceptTournamentInvitation(r.Context(), auth.Account(r.Context()), id, boardID)
	}
	if err != nil && !isNotFound(err) {
		c.joinFailed(w, r, err, view.T(r.Context()).ErrInput(), false)
		return
	}
	http.Redirect(w, r, tournamentURL(id), http.StatusSeeOther)
}

func (c *Controller) declineTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := c.svc.DeclineTournamentInvitation(r.Context(), auth.Account(r.Context()), id); err != nil && !isNotFound(err) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
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
		c.renderTournament(w, r, http.StatusConflict, view.T(r.Context()).ErrInput())
	default:
		serverError(w, err)
	}
}
