package controller

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/sj14/darts-o-mat/common/sse"
	"github.com/sj14/darts-o-mat/common/throw"
	"github.com/sj14/darts-o-mat/server/auth"
	"github.com/sj14/darts-o-mat/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/server/frontend/view"
	"github.com/sj14/darts-o-mat/server/service"
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

// gameEvents streams the board to the game page whenever the game changes, as
// Server-Sent Events ("board"), e.g. when a dart comes from the board's cameras or
// another player's device. The page swaps it in with the htmx SSE extension.
func (c *Controller) gameEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	acc := auth.Account(r.Context())
	if _, err := c.svc.Game(r.Context(), acc, id); isNotFound(err) {
		http.NotFound(w, r)
		return
	} else if err != nil {
		serverError(w, err)
		return
	}
	changes, stop := c.svc.Subscribe(id)
	defer stop()
	sse.Stream(w, r, "board", changes, func() (string, error) {
		v, err := c.svc.Game(r.Context(), acc, id)
		if err != nil {
			return "", err // e.g. the game was deleted because everyone declined
		}
		var buf bytes.Buffer
		if err := view.Board(v, "").Render(r.Context(), &buf); err != nil {
			log.Printf("render board: %v", err)
			return "", err
		}
		return buf.String(), nil
	})
}

// addDart is called by htmx and responds with the updated board.
func (c *Controller) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}

	d, ok := dartFromForm(r)
	if !ok {
		c.renderBoard(w, r, id, view.T(r.Context()).ErrInput())
		return
	}

	v, err := c.svc.AddDart(r.Context(), auth.Account(r.Context()), id, d)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case d.Number == throw.BullNumber && d.Segment == throw.Triple:
		c.renderBoard(w, r, id, view.T(r.Context()).ErrTripleBull())
	case errors.Is(err, throw.ErrInvalid):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNoSuchField())
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
	c.respondBoard(w, r, id, v, err)
}

// showBoard returns the board, e.g. to cancel correcting a dart.
func (c *Controller) showBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	c.renderBoard(w, r, id, "")
}

// leaveGame is called when an account leaves a game: back to the start page
// from the lobby, else to the game, which they can still look at.
func (c *Controller) leaveGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.LeaveGame(r.Context(), auth.Account(r.Context()), id)
	switch {
	case err != nil:
		c.handleGameError(w, r, id, err)
	case v.Game.ID == 0:
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Redirect(w, r, gameURL(id), http.StatusSeeOther)
	}
}

// showKeypad returns the board with the keypad to enter a dart the cameras missed.
func (c *Controller) showKeypad(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.Game(r.Context(), auth.Account(r.Context()), id)
	if err != nil {
		c.handleGameError(w, r, id, err)
		return
	}
	render(w, r, http.StatusOK, view.BoardAdding(v, ""))
}

// showEditDart returns the board with the keypad to correct a dart.
func (c *Controller) showEditDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	dartID, ok := urlID(w, r, "dartID")
	if !ok {
		return
	}
	v, err := c.svc.Game(r.Context(), auth.Account(r.Context()), id)
	if err != nil {
		c.handleGameError(w, r, id, err)
		return
	}
	i := slices.IndexFunc(v.Darts, func(d throw.Dart) bool { return d.ID == dartID && d.IsDart() })
	if i < 0 {
		http.NotFound(w, r)
		return
	}
	if !v.CanThrow || !v.MayEdit(i) {
		c.handleGameError(w, r, id, service.ErrForbidden)
		return
	}
	render(w, r, http.StatusOK, view.BoardEditing(v, "", v.Darts[i]))
}

// editDart saves the correction of a dart.
func (c *Controller) editDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	dartID, ok := urlID(w, r, "dartID")
	if !ok {
		return
	}
	d, ok := dartFromForm(r)
	if !ok {
		c.renderBoard(w, r, id, view.T(r.Context()).ErrInput())
		return
	}
	v, err := c.svc.EditDart(r.Context(), auth.Account(r.Context()), id, dartID, d)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case errors.Is(err, service.ErrChangesThrowers):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrChangesThrowers())
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrEndsEarlier())
	case d.Number == throw.BullNumber && d.Segment == throw.Triple:
		c.renderBoard(w, r, id, view.T(r.Context()).ErrTripleBull())
	case errors.Is(err, throw.ErrInvalid):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNoSuchField())
	default:
		c.handleGameError(w, r, id, err)
	}
}

// takeout is called by htmx when the players confirm that the darts were pulled out.
func (c *Controller) takeout(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.Takeout(r.Context(), auth.Account(r.Context()), id)
	c.respondBoard(w, r, id, v, err)
}

// complete is called by htmx when a player confirms the result of the finished game.
func (c *Controller) complete(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.CompleteGame(r.Context(), auth.Account(r.Context()), id)
	c.respondBoard(w, r, id, v, err)
}

// decideBullOff is called by htmx when the players decide who was closer in a bull-off tie.
func (c *Controller) decideBullOff(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	playerID, ok := urlID(w, r, "playerID")
	if !ok {
		return
	}
	v, err := c.svc.DecideBullOff(r.Context(), auth.Account(r.Context()), id, playerID)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case errors.Is(err, service.ErrInvalid):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNoBullOffTie())
	default:
		c.handleGameError(w, r, id, err)
	}
}

// useBoard is called by htmx when a player puts their players of the game on a board.
func (c *Controller) useBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, err := boardFromForm(r) // empty: no board
	if err != nil {
		c.renderBoard(w, r, id, view.T(r.Context()).ErrInput())
		return
	}
	v, err := c.svc.UseBoard(r.Context(), auth.Account(r.Context()), id, boardID)
	c.respondBoard(w, r, id, v, err)
}

func (c *Controller) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, err := boardFromForm(r)
	if err == nil {
		_, err = c.svc.AcceptInvitation(r.Context(), auth.Account(r.Context()), id, boardID)
	}
	if err != nil && !isNotFound(err) {
		c.joinFailed(w, r, err, view.T(r.Context()).ErrInput(), false)
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
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNoInvitation())
	default:
		c.handleGameError(w, r, id, err)
	}
}

// respondBoard responds to an action on a game with the board, or with the error.
func (c *Controller) respondBoard(w http.ResponseWriter, r *http.Request, id int64, v service.GameView, err error) {
	if err != nil {
		c.handleGameError(w, r, id, err)
		return
	}
	render(w, r, http.StatusOK, view.Board(v, ""))
}

// handleGameError responds to errors shared by the actions on a game.
func (c *Controller) handleGameError(w http.ResponseWriter, r *http.Request, id int64, err error) {
	switch {
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrGameOver())
	case errors.Is(err, turn.ErrWaitingForTakeout):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrPullDartsFirst())
	case errors.Is(err, service.ErrNotReady):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNotReady())
	case errors.Is(err, service.ErrAborted):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrAborted())
	case errors.Is(err, service.ErrCompleted):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrCompleted())
	case errors.Is(err, service.ErrNotOver):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNotOver())
	case errors.Is(err, service.ErrNothingToUndo):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNothingToUndo())
	case errors.Is(err, service.ErrCantLeave):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrCantLeave())
	case errors.Is(err, service.ErrForbidden):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrForbidden())
	case errors.Is(err, service.ErrInvalid):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrInput())
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

// dartFromForm reads the dart entered with the keypad.
func dartFromForm(r *http.Request) (throw.Dart, bool) {
	if r.FormValue("miss") == "true" {
		return throw.Dart{Segment: throw.Miss}, true
	}
	number, err1 := strconv.ParseUint(r.FormValue("number"), 10, 8)
	segment, err2 := throw.ParseSegment(r.FormValue("segment"))
	if err1 != nil || err2 != nil {
		return throw.Dart{}, false
	}
	d := throw.Dart{Number: uint8(number), Segment: segment}
	if d.Number == throw.BullNumber {
		// The keypad's bull key follows the single/double choice.
		switch segment {
		case throw.Single:
			d.Segment = throw.Bull
		case throw.Double:
			d.Segment = throw.Bullseye
		}
	}
	return d, true
}
