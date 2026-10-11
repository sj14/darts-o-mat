// Package api provides the JSON API. Except for register and login,
// all endpoints need the session cookie set by login.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/service"
)

type API struct {
	svc *service.Service
}

func New(svc *service.Service) *API {
	return &API{svc: svc}
}

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/register", a.register)
	r.Post("/login", a.login)
	r.Post("/logout", a.logout)

	r.Group(func(r chi.Router) {
		r.Use(auth.Require(a.svc, func(w http.ResponseWriter, r *http.Request) {
			writeError(w, service.ErrUnauthenticated)
		}))

		r.Get("/players", a.listPlayers)
		r.Post("/players", a.createGuest)
		r.Get("/boards", a.listBoards)
		r.Post("/boards", a.createBoard)
		r.Delete("/boards/{id}", a.deleteBoard)

		r.Get("/games", a.listGames)
		r.Post("/games", a.createGame)
		r.Get("/games/{id}", a.getGame)
		r.Post("/games/{id}/darts", a.addDart)
		r.Delete("/games/{id}/darts/last", a.undoDart)

		r.Get("/stats", a.stats)
		r.Get("/friends", a.listFriends)
		r.Post("/friends", a.requestFriend)
		r.Post("/friends/{accountID}/accept", a.acceptFriend)
		r.Delete("/friends/{accountID}", a.removeFriend)

		r.Get("/invitations", a.listInvitations)
		r.Post("/games/{id}/accept", a.acceptInvitation)
		r.Post("/games/{id}/decline", a.declineInvitation)
		r.Delete("/games/{id}/invitations/{playerID}", a.withdrawInvitation)
	})
	return r
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	acc, err := a.svc.Register(r.Context(), req.Username, req.Name, req.Password)
	respond(w, http.StatusCreated, acc, err)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	acc, err := auth.Login(w, r, a.svc, req.Username, req.Password)
	respond(w, http.StatusOK, acc, err)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if err := auth.Logout(w, r, a.svc); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := a.svc.Players(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, players, err)
}

func (a *API) createGuest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	p, err := a.svc.CreateGuest(r.Context(), auth.Account(r.Context()), req.Name)
	respond(w, http.StatusCreated, p, err)
}

func (a *API) listBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := a.svc.Boards(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, boards, err)
}

func (a *API) createBoard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	b, err := a.svc.CreateBoard(r.Context(), auth.Account(r.Context()), req.Name)
	respond(w, http.StatusCreated, b, err)
}

func (a *API) deleteBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteBoard(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := a.svc.ListGames(r.Context(), auth.Account(r.Context()), 50)
	respond(w, http.StatusOK, games, err)
}

func (a *API) createGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		game.Game
		// Invite lists usernames of other accounts to invite; they are added after PlayerIDs.
		Invite []string `json:"invite"`
	}
	if !decode(w, r, &req) {
		return
	}
	v, err := a.svc.CreateGame(r.Context(), auth.Account(r.Context()), req.Game, req.Invite)
	respond(w, http.StatusCreated, v, err)
}

func (a *API) getGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.Game(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

func (a *API) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	var d throw.Dart
	if !decode(w, r, &d) {
		return
	}
	v, err := a.svc.AddDart(r.Context(), auth.Account(r.Context()), id, d)
	respond(w, http.StatusCreated, v, err)
}

func (a *API) undoDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.UndoDart(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

// stats returns the statistics of the account's player and its guests over all finished games.
func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	summaries, err := a.svc.Stats(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, summaries, err)
}

func (a *API) listFriends(w http.ResponseWriter, r *http.Request) {
	friends, err := a.svc.Friends(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, friends, err)
}

// requestFriend sends a friend request, or accepts the request the other account already sent.
func (a *API) requestFriend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.svc.RequestFriend(r.Context(), auth.Account(r.Context()), req.Username); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) acceptFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "accountID")
	if !ok {
		return
	}
	if err := a.svc.AcceptFriend(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeFriend ends a friendship, or declines or withdraws a request.
func (a *API) removeFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "accountID")
	if !ok {
		return
	}
	if err := a.svc.RemoveFriend(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listInvitations(w http.ResponseWriter, r *http.Request) {
	games, err := a.svc.Invitations(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, games, err)
}

func (a *API) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.AcceptInvitation(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

func (a *API) declineInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := a.svc.DeclineInvitation(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) withdrawInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	playerID, ok := urlID(w, r, "playerID")
	if !ok {
		return
	}
	v, err := a.svc.WithdrawInvitation(r.Context(), auth.Account(r.Context()), id, playerID)
	respond(w, http.StatusOK, v, err)
}

// urlID parses a numeric URL parameter and writes the error response if that fails.
func urlID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid %s", service.ErrInvalid, name))
		return 0, false
	}
	return id, true
}

// decode reads the JSON body into v and writes the error response if that fails.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return false
	}
	return true
}

func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrAlreadyFriends):
		status = http.StatusConflict
	case errors.Is(err, service.ErrInvalid), errors.Is(err, throw.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrUnauthenticated):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrRegistrationClosed):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, turn.ErrGameOver), errors.Is(err, service.ErrNothingToUndo),
		errors.Is(err, service.ErrNotReady), errors.Is(err, service.ErrUsernameTaken):
		status = http.StatusConflict
	}

	msg := err.Error()
	if status == http.StatusInternalServerError {
		log.Printf("api: %v", err)
		msg = http.StatusText(status)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
