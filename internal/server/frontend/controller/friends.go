package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func (c *Controller) showFriends(w http.ResponseWriter, r *http.Request) {
	c.renderFriends(w, r, http.StatusOK, "", "")
}

func (c *Controller) renderFriends(w http.ResponseWriter, r *http.Request, status int, errMsg, info string) {
	acc := auth.Account(r.Context())
	list, err := c.svc.Friends(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Friends(acc.Username, list, errMsg, info))
}

func (c *Controller) requestFriend(w http.ResponseWriter, r *http.Request) {
	err := c.svc.RequestFriend(r.Context(), auth.Account(r.Context()), r.FormValue("username"))
	t := view.T(r.Context())
	switch msg, ok := errorText(t, err); {
	case err == nil:
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
	case ok:
		c.renderFriends(w, r, http.StatusBadRequest, msg, "")
	case errors.Is(err, service.ErrInvalid):
		c.renderFriends(w, r, http.StatusBadRequest, t.ErrFriendSelf(), "")
	default:
		serverError(w, err)
	}
}

func (c *Controller) acceptFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := c.svc.AcceptFriend(r.Context(), auth.Account(r.Context()), id)
	switch msg, ok := errorText(view.T(r.Context()), err); {
	case err == nil, isNotFound(err):
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
	case ok:
		c.renderFriends(w, r, http.StatusBadRequest, msg, "")
	default:
		serverError(w, err)
	}
}

// removeFriend ends a friendship, or declines or withdraws a request.
func (c *Controller) removeFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := c.svc.RemoveFriend(r.Context(), auth.Account(r.Context()), id); err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
}
