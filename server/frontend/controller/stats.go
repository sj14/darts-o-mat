package controller

import (
	"net/http"

	"github.com/sj14/darts-o-mat/server/auth"
	"github.com/sj14/darts-o-mat/server/frontend/view"
)

func (c *Controller) showStats(w http.ResponseWriter, r *http.Request) {
	acc := auth.Account(r.Context())
	summaries, err := c.svc.Stats(r.Context(), acc)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Stats(acc.Username, summaries))
}
