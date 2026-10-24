package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
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

// quickMatch puts the account's own player into a quick match of the chosen mode and goes
// to its page: a game that starts at once, or one where they wait for an opponent.
func (c *Controller) quickMatch(w http.ResponseWriter, r *http.Request) {
	boardID, err := boardFromForm(r)
	var v service.GameView
	if err == nil {
		v, err = c.svc.QuickMatch(r.Context(), auth.Account(r.Context()), game.Mode(r.FormValue("mode")), boardID)
	}
	if err != nil {
		c.respondLobby(w, r, service.GameLobby(v.Game.ID), err, true) // the message on the page of the open games
		return
	}
	http.Redirect(w, r, gameURL(v.Game.ID), http.StatusSeeOther)
}

// quickMatchMove is asked by the page of a quick match that waits for an opponent: if the
// player moved to an older one that fits by now, htmx goes to its page; else the page asks
// again once the next one will fit (a second late, as created_at keeps whole seconds and
// the clocks of database and server may differ a little), or not at all.
func (c *Controller) quickMatchMove(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	moved, next, err := c.svc.QuickMatchMove(r.Context(), auth.Account(r.Context()), id)
	switch {
	case isNotFound(err):
		http.NotFound(w, r)
	case err != nil:
		serverError(w, err)
	case moved != id:
		w.Header().Set("HX-Redirect", gameURL(moved))
		w.WriteHeader(http.StatusNoContent)
	case next.IsZero():
		w.WriteHeader(http.StatusNoContent)
	default:
		render(w, r, http.StatusOK, view.QuickMatchCheck(id, max(time.Until(next), 0)+time.Second))
	}
}
