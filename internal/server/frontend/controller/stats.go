package controller

import (
	"net/http"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
)

func (c *Controller) showStats(w http.ResponseWriter, r *http.Request) {
	acc := auth.Account(r.Context())
	mode := game.Mode(r.URL.Query().Get("mode"))
	if mode != game.ModeX01 && mode != game.ModeAroundTheClock {
		mode = ""
	}
	summaries, err := c.svc.Stats(r.Context(), acc, mode)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Stats(acc.Username, summaries, mode))
}
