// Package api provides the JSON API to manage players and play games.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
	r.Get("/players", a.listPlayers)
	r.Post("/players", a.createPlayer)
	r.Post("/games", a.createGame)
	r.Get("/games/{id}", a.getGame)
	r.Post("/games/{id}/darts", a.addDart)
	r.Delete("/games/{id}/darts/last", a.undoDart)
	return r
}

func (a *API) listPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := a.svc.ListPlayers(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, players)
}

func (a *API) createPlayer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return
	}

	p, err := a.svc.CreatePlayer(r.Context(), req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) createGame(w http.ResponseWriter, r *http.Request) {
	var g game.Game
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return
	}

	v, err := a.svc.CreateGame(r.Context(), g)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) getGame(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}
	v, err := a.svc.Game(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (a *API) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}

	var d throw.Dart
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return
	}

	v, err := a.svc.AddDart(r.Context(), id, d)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

func (a *API) undoDart(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}
	v, err := a.svc.UndoDart(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// gameID parses the {id} URL parameter and writes the error response if that fails.
func gameID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid game id", service.ErrInvalid))
		return 0, false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrInvalid), errors.Is(err, throw.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, turn.ErrGameOver), errors.Is(err, service.ErrNothingToUndo):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
