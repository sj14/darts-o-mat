package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) showGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	acc := auth.Account(r.Context())
	v, err := c.svc.Game(r.Context(), acc, id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Game(acc.Username, v))
}

// showBoard returns only the board, used by the game page to refresh itself.
// If the board still is the version the page shows (?v=), it responds 204 No Content.
func (c *Controller) showBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.Game(r.Context(), auth.Account(r.Context()), id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	if r.URL.Query().Get("v") == view.BoardVersion(v) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	render(w, r, http.StatusOK, view.Board(v, ""))
}

// addDart is called by htmx and responds with the updated board.
func (c *Controller) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
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

	v, err := c.svc.AddDart(r.Context(), auth.Account(r.Context()), id, d)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case d.Number == throw.Bull && d.Multiplier == 3:
		c.renderBoard(w, r, id, "Es gibt kein Triple-Bull.")
	case errors.Is(err, throw.ErrInvalid):
		c.renderBoard(w, r, id, "Diesen Treffer gibt es nicht.")
	default:
		c.handleGameError(w, r, id, err)
	}
}

func (c *Controller) undoDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}

	v, err := c.svc.UndoDart(r.Context(), auth.Account(r.Context()), id)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case errors.Is(err, service.ErrNothingToUndo):
		c.renderBoard(w, r, id, "Es gibt keinen Dart zum Zurücknehmen.")
	default:
		c.handleGameError(w, r, id, err)
	}
}

func (c *Controller) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	_, err := c.svc.AcceptInvitation(r.Context(), auth.Account(r.Context()), id)
	if err != nil && !isNotFound(err) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, gameURL(id), http.StatusSeeOther)
}

func (c *Controller) declineInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := c.svc.DeclineInvitation(r.Context(), auth.Account(r.Context()), id)
	if err != nil && !isNotFound(err) {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// withdrawInvitation is called by htmx from the lobby and responds with the updated board.
func (c *Controller) withdrawInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	playerID, ok := urlID(w, r, "playerID")
	if !ok {
		return
	}

	v, err := c.svc.WithdrawInvitation(r.Context(), auth.Account(r.Context()), id, playerID)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case isNotFound(err):
		// No player left, the game was deleted.
		w.Header().Set("HX-Redirect", "/")
	case errors.Is(err, service.ErrInvalid):
		c.renderBoard(w, r, id, "Diese Einladung gibt es nicht mehr.")
	default:
		c.handleGameError(w, r, id, err)
	}
}

// handleGameError responds to errors shared by the actions on a game.
func (c *Controller) handleGameError(w http.ResponseWriter, r *http.Request, id int64, err error) {
	switch {
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, "Das Spiel ist schon vorbei.")
	case errors.Is(err, service.ErrNotReady):
		c.renderBoard(w, r, id, "Es haben noch nicht alle Eingeladenen zugesagt.")
	case errors.Is(err, service.ErrForbidden):
		c.renderBoard(w, r, id, "Das darfst du in diesem Spiel nicht.")
	case isNotFound(err):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

// renderBoard shows the current board with a message. It responds with 200,
// because htmx does not swap in error responses by default.
func (c *Controller) renderBoard(w http.ResponseWriter, r *http.Request, id int64, msg string) {
	v, err := c.svc.Game(r.Context(), auth.Account(r.Context()), id)
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

func gameURL(id int64) string {
	return "/games/" + strconv.FormatInt(id, 10)
}
