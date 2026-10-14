package controller

import (
	"errors"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
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
	var unknown service.UnknownUserError
	switch {
	case err == nil:
		http.Redirect(w, r, "/friends", http.StatusSeeOther)
	case errors.As(err, &unknown):
		c.renderFriends(w, r, http.StatusBadRequest, view.T(r.Context()).ErrUnknownUser(unknown.Username), "")
	case errors.Is(err, service.ErrAlreadyFriends):
		c.renderFriends(w, r, http.StatusConflict, view.T(r.Context()).ErrAlreadyFriends(), "")
	case errors.Is(err, service.ErrInvalid):
		c.renderFriends(w, r, http.StatusBadRequest, view.T(r.Context()).ErrFriendSelf(), "")
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
	if err != nil && !isNotFound(err) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/friends", http.StatusSeeOther)
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
