package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
)

type GameX01Model struct {
	Game x01.GameX01
}

func (c *Controller) ShowNewGameX01(w http.ResponseWriter, r *http.Request) {
	players, err := c.repo.GetPlayers(r.Context(), []int{0, 1, 2})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view.ShowCreateX01(players).Render(r.Context(), w)
}

func (c *Controller) PostNewGameX01(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("%+v\n", r.Form)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	counterStr := r.FormValue("hit_counter")

	counter, err := strconv.ParseUint(counterStr, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	playerIDsStr := r.Form["players"]
	var playerIDs []int

	for _, idStr := range playerIDsStr {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		playerIDs = append(playerIDs, id)
	}

	players, err := c.repo.GetPlayers(r.Context(), playerIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	g, err := x01.NewX01(counter, players)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	model := GameX01Model{
		Game: g,
	}

	view.ShowX01(model.Game.ID).Render(r.Context(), w)
}
