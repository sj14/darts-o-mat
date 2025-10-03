package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"gorm.io/gorm"
)

type GameX01Model struct {
	Game game.GameX01
}

func (controller *Controller) ShowNewGameX01(w http.ResponseWriter, r *http.Request) {
	p1 := player.Player{Model: gorm.Model{ID: 0}, Name: "Max"}
	p2 := player.Player{Model: gorm.Model{ID: 1}, Name: "Jan"}

	view.ShowCreateX01([]player.Player{p1, p2}).Render(r.Context(), w)
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

	g, err := game.NewX01(counter, players)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	model := GameX01Model{
		Game: g,
	}

	view.ShowX01(model.Game.GameID).Render(r.Context(), w)
}
