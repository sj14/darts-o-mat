package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// lobbyRoutes adds what games and tournaments both do in their lobby, under the prefix
// (/games, /tournaments); ref names the one of the URL's id. They are plain forms: each
// goes back to the page of the game or tournament, the others see the change live.
func (c *Controller) lobbyRoutes(r chi.Router, prefix string, ref func(int64) service.LobbyRef) {
	type change func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64) error
	do := func(fromLobbies bool, change change) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, ok := urlID(w, r, "id")
			if !ok {
				return
			}
			var playerID int64
			if chi.URLParam(r, "playerID") != "" {
				if playerID, ok = urlID(w, r, "playerID"); !ok {
					return
				}
			}
			l := ref(id)
			err := change(r, auth.Account(r.Context()), l, playerID)
			c.respondLobby(w, r, l, err, fromLobbies)
		}
	}
	withBoard := func(f func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64, boardID *int64) error) change {
		return func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64) error {
			boardID, err := boardFromForm(r)
			if err != nil {
				return err
			}
			return f(r, acc, l, playerID, boardID)
		}
	}
	r.Post(prefix+"/{id}/players", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64) error {
		playerID, err := c.playerFromForm(r, acc)
		if err != nil {
			return err
		}
		return c.svc.AddPlayer(r.Context(), acc, l, playerID)
	}))
	r.Post(prefix+"/{id}/players/{playerID}/remove", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64) error {
		return c.svc.RemovePlayer(r.Context(), acc, l, playerID)
	}))
	r.Post(prefix+"/{id}/players/{playerID}/board", do(false, withBoard(func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64, boardID *int64) error {
		return c.svc.SetSeatBoard(r.Context(), acc, l, playerID, boardID)
	})))
	r.Post(prefix+"/{id}/invitations", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64) error {
		return c.svc.Invite(r.Context(), acc, l, r.FormValue("username"))
	}))
	r.Post(prefix+"/{id}/invitations/{playerID}/withdraw", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, playerID int64) error {
		return c.svc.Withdraw(r.Context(), acc, l, playerID)
	}))
	r.Post(prefix+"/{id}/accept", do(false, withBoard(func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64, boardID *int64) error {
		return c.svc.Accept(r.Context(), acc, l, boardID)
	})))
	r.Post(prefix+"/{id}/decline", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64) error {
		return c.svc.Decline(r.Context(), acc, l)
	}))
	r.Post(prefix+"/{id}/join", do(true, withBoard(func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64, boardID *int64) error {
		return c.svc.Join(r.Context(), acc, l, boardID)
	})))
	r.Post(prefix+"/{id}/start", do(false, func(r *http.Request, acc account.Account, l service.LobbyRef, _ int64) error {
		return c.svc.Start(r.Context(), acc, l)
	}))
}

// playerFromForm reads the player to add to a lobby: a guest made of the name entered, or
// one of the account's players chosen.
func (c *Controller) playerFromForm(r *http.Request, acc account.Account) (int64, error) {
	if name := strings.TrimSpace(r.FormValue("name")); name != "" {
		guest, err := c.svc.CreateGuest(r.Context(), acc, name)
		return guest.ID, err
	}
	id, err := strconv.ParseInt(r.FormValue("player_id"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: choose a player or enter a name", service.ErrInvalid)
	}
	return id, nil
}

// respondLobby answers a change of a lobby: its page again, the start page if the account
// has nothing to see there anymore (declined, left), or with the message why it failed:
// on the page of the open games for a join, else on the lobby's page.
func (c *Controller) respondLobby(w http.ResponseWriter, r *http.Request, l service.LobbyRef, err error, fromLobbies bool) {
	page := gameURL(l.ID)
	if l.Tournament {
		page = tournamentURL(l.ID)
	}
	if err == nil {
		if !c.sees(r, l) {
			page = "/"
		}
		http.Redirect(w, r, page, http.StatusSeeOther)
		return
	}
	t := view.T(r.Context())
	msg, ok := errorText(t, err)
	switch {
	case isNotFound(err):
		http.NotFound(w, r)
		return
	case !ok && !errors.Is(err, service.ErrInvalid):
		serverError(w, err)
		return
	case !ok:
		msg = t.ErrInput()
	}
	failed := alertOf(err, msg)
	switch {
	case fromLobbies:
		c.renderLobbies(w, r, http.StatusConflict, failed)
	case l.Tournament:
		c.renderTournament(w, r, http.StatusConflict, failed)
	default:
		c.renderGame(w, r, http.StatusConflict, failed.Text)
	}
}

// sees reports whether the account still takes part in the game or tournament.
func (c *Controller) sees(r *http.Request, l service.LobbyRef) bool {
	var err error
	if l.Tournament {
		_, err = c.svc.Tournament(r.Context(), auth.Account(r.Context()), l.ID)
	} else {
		_, err = c.svc.Game(r.Context(), auth.Account(r.Context()), l.ID)
	}
	return err == nil
}
