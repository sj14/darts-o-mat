package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showBoards(w http.ResponseWriter, r *http.Request) {
	c.renderBoards(w, r, http.StatusOK, "")
}

func (c *Controller) renderBoards(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
	acc := auth.Account(r.Context())
	boards, err := c.svc.Boards(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Boards(acc.Username, boards, errMsg))
}

func (c *Controller) createBoard(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.CreateBoard(r.Context(), auth.Account(r.Context()), r.FormValue("name"))
	if errors.Is(err, service.ErrInvalid) {
		c.renderBoards(w, r, http.StatusBadRequest, "Bitte einen Namen eingeben.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/boards", http.StatusSeeOther)
}

func (c *Controller) deleteBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := c.svc.DeleteBoard(r.Context(), auth.Account(r.Context()), id)
	if err != nil && !isNotFound(err) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/boards", http.StatusSeeOther)
}
