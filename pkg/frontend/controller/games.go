package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showGame(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}
	v, err := c.svc.Game(r.Context(), id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Game(v))
}

// addDart is called by htmx and responds with the updated board.
func (c *Controller) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}

	d := throw.Dart{Miss: r.FormValue("miss") == "true"}
	if !d.Miss {
		number, err1 := strconv.ParseUint(r.FormValue("number"), 10, 8)
		multiplier, err2 := strconv.ParseUint(r.FormValue("multiplier"), 10, 8)
		if err1 != nil || err2 != nil {
			c.renderBoard(w, r, id, "Ungültige Eingabe.")
			return
		}
		d.Number, d.Multiplier = uint8(number), uint8(multiplier)
	}

	v, err := c.svc.AddDart(r.Context(), id, d)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case d.Number == throw.Bull && d.Multiplier == 3:
		c.renderBoard(w, r, id, "Es gibt kein Triple-Bull.")
	case errors.Is(err, throw.ErrInvalid):
		c.renderBoard(w, r, id, "Diesen Treffer gibt es nicht.")
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, "Das Spiel ist schon vorbei.")
	case isNotFound(err):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

func (c *Controller) undoDart(w http.ResponseWriter, r *http.Request) {
	id, ok := gameID(w, r)
	if !ok {
		return
	}

	v, err := c.svc.UndoDart(r.Context(), id)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case errors.Is(err, service.ErrNothingToUndo):
		c.renderBoard(w, r, id, "Es gibt keinen Dart zum Zurücknehmen.")
	case isNotFound(err):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

// renderBoard shows the current board with a message. It responds with 200,
// because htmx does not swap in error responses by default.
func (c *Controller) renderBoard(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	v, err := c.svc.Game(r.Context(), id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Board(v, msg))
}
