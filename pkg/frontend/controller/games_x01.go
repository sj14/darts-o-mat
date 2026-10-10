package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
)

func (c *Controller) ShowNewGameX01(w http.ResponseWriter, r *http.Request) {
	players, err := c.repo.ListPlayers(r.Context())
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
	var playerIDs []int64

	for _, idStr := range playerIDsStr {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		playerIDs = append(playerIDs, id)
	}

	g := game.Game{
		Mode:      game.ModeX01,
		PlayerIDs: playerIDs,
		X01:       &x01.Settings{StartScore: counter, OutMode: x01.OutModeDouble},
	}
	if err := g.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := c.repo.CreateGame(r.Context(), &g); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	view.ShowX01(g.ID).Render(r.Context(), w)
}
