package controller

import (
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
