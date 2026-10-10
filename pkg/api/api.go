// Package api provides the JSON API to manage players and play games.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

type API struct {
	store *store.Store
	// dartsMu serializes changes to the darts, so two darts can't both pass
	// the replay check against the same previous state.
	dartsMu sync.Mutex
}

func New(store *store.Store) *API {
	return &API{store: store}
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

// gameResponse is returned by all game endpoints.
type gameResponse struct {
	Game  game.Game    `json:"game"`
	Darts []throw.Dart `json:"darts"`
	State any          `json:"state"` // x01.State or aroundtheclock.State, depending on the mode
}

func (a *API) listPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := a.store.ListPlayers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, players)
}

func (a *API) createPlayer(w http.ResponseWriter, r *http.Request) {
	var p player.Player
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p.ID = 0
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("name must not be empty"))
		return
	}

	if err := a.store.StorePlayer(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (a *API) createGame(w http.ResponseWriter, r *http.Request) {
	var g game.Game
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	g.ID = 0
	if err := g.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	players, err := a.store.GetPlayers(r.Context(), g.PlayerIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(players) != len(g.PlayerIDs) {
		writeError(w, http.StatusBadRequest, errors.New("unknown player in player_ids"))
		return
	}

	if err := a.store.CreateGame(r.Context(), &g); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	state, err := g.Play(nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, gameResponse{Game: g, Darts: []throw.Dart{}, State: state})
}

func (a *API) getGame(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadGame(w, r)
	if !ok {
		return
	}
	a.respondGame(w, r, http.StatusOK, g)
}

// addDart registers the next dart of the game. The dart is only stored
// if the game accepts it, i.e. it is valid and the game is not over yet.
func (a *API) addDart(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadGame(w, r)
	if !ok {
		return
	}

	var d throw.Dart
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	d.ID = 0
	d.GameID = g.ID
	if d.Miss {
		d.Number, d.Multiplier = 0, 0
	} else if d.Multiplier == 0 {
		d.Multiplier = 1
	}

	a.dartsMu.Lock()
	defer a.dartsMu.Unlock()

	darts, err := a.store.ListDarts(r.Context(), g.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// The stored darts were accepted before, so an error can only come from the new one.
	if _, err := g.Play(append(darts, d)); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, turn.ErrGameOver) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}

	if err := a.store.AddDart(r.Context(), &d); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.respondGame(w, r, http.StatusCreated, g)
}

func (a *API) undoDart(w http.ResponseWriter, r *http.Request) {
	g, ok := a.loadGame(w, r)
	if !ok {
		return
	}

	a.dartsMu.Lock()
	defer a.dartsMu.Unlock()

	err := a.store.DeleteLastDart(r.Context(), g.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, errors.New("no dart to undo"))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	a.respondGame(w, r, http.StatusOK, g)
}

// loadGame loads the game from the {id} URL parameter and writes the error response if that fails.
func (a *API) loadGame(w http.ResponseWriter, r *http.Request) (game.Game, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid game id: %w", err))
		return game.Game{}, false
	}

	g, err := a.store.GetGame(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, errors.New("game not found"))
		return game.Game{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return game.Game{}, false
	}
	return g, true
}

func (a *API) respondGame(w http.ResponseWriter, r *http.Request, status int, g game.Game) {
	darts, err := a.store.ListDarts(r.Context(), g.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	state, err := g.Play(darts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, status, gameResponse{Game: g, Darts: darts, State: state})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
