package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showBoards(w http.ResponseWriter, r *http.Request) {
	c.renderBoards(w, r, http.StatusOK, "", nil)
}

func (c *Controller) renderBoards(w http.ResponseWriter, r *http.Request, status int, errMsg string, newToken *view.NewBoardToken) {
	acc := auth.Account(r.Context())
	boards, err := c.svc.Boards(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Boards(acc.Username, boards, errMsg, newToken))
}

// createBoardToken creates a token for the board's cameras and shows it once.
func (c *Controller) createBoardToken(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	acc := auth.Account(r.Context())
	token, err := c.svc.CreateBoardToken(r.Context(), acc, id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}

	name := ""
	if boards, err := c.svc.Boards(r.Context(), acc); err == nil {
		for _, b := range boards {
			if b.ID == id {
				name = b.Name
			}
		}
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	c.renderBoards(w, r, http.StatusOK, "", &view.NewBoardToken{BoardName: name, Token: token, Server: scheme + "://" + r.Host})
}

func (c *Controller) createBoard(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.CreateBoard(r.Context(), auth.Account(r.Context()), r.FormValue("name"))
	if errors.Is(err, service.ErrInvalid) {
		c.renderBoards(w, r, http.StatusBadRequest, "Bitte einen Namen eingeben.", nil)
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
