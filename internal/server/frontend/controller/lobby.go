package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// showLobbies lists the open games and tournaments, narrowed down by the filter of the query.
func (c *Controller) showLobbies(w http.ResponseWriter, r *http.Request) {
	c.renderLobbies(w, r, http.StatusOK, view.Alert{})
}

// renderLobbies shows the page of the open games, with the message if there is one.
func (c *Controller) renderLobbies(w http.ResponseWriter, r *http.Request, status int, failed view.Alert) {
	ctx := r.Context()
	acc := auth.Account(ctx)
	lobbies, err := c.svc.OpenLobbies(ctx, acc)
	if err != nil {
		serverError(w, err)
		return
	}
	boards, err := c.svc.Boards(ctx, acc)
	if err != nil {
		serverError(w, err)
		return
	}
	filter := service.LobbyFilterFromQuery(r.URL.Query())
	render(w, r, status, view.Lobbies(view.LobbiesModel{
		Account: acc,
		Lobbies: filter.Apply(lobbies),
		Total:   len(lobbies),
		Filter:  filter,
		Boards:  boards,
		Error:   failed,
	}))
}

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
	if c.joinFailed(w, r, err, view.T(r.Context()).ErrNoSeat(), true) {
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
	if c.joinFailed(w, r, err, view.T(r.Context()).ErrNoSeat(), true) {
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

// joinFailed answers a failed join or accept, if it failed: the page it came from with
// the message, the open games for a join, else the start page.
func (c *Controller) joinFailed(w http.ResponseWriter, r *http.Request, err error, msg string, fromLobbies bool) bool {
	if text, ok := errorText(view.T(r.Context()), err); ok {
		msg = text
	}
	switch {
	case err == nil:
		return false
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrInvalid) && fromLobbies:
		c.renderLobbies(w, r, http.StatusConflict, view.Alert{Text: msg, Game: busyGame(err)})
	case errors.Is(err, service.ErrInvalid):
		c.renderHome(w, r, http.StatusConflict, "", view.Alert{Text: msg, Game: busyGame(err)})
	default:
		serverError(w, err)
	}
	return true
}
