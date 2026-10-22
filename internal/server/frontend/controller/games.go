package controller

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/sse"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func (c *Controller) showGame(w http.ResponseWriter, r *http.Request) {
	c.renderGame(w, r, http.StatusOK, "")
}

// renderGame shows the page of the game of the URL, with the message if there is one.
func (c *Controller) renderGame(w http.ResponseWriter, r *http.Request, status int, errMsg string) {
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
	render(w, r, status, view.Game(acc.Username, v, errMsg))
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
	if err != nil {
		c.handleDartError(w, r, id, d, err)
		return
	}
	render(w, r, http.StatusOK, view.Board(v, ""))
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
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrEndsEarlier())
	default:
		c.handleDartError(w, r, id, d, err)
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

// ratePlayer is called by htmx when a player gives another a thumb up (up=1) or down.
func (c *Controller) ratePlayer(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	playerID, ok := urlID(w, r, "playerID")
	if !ok {
		return
	}
	v, err := c.svc.RatePlayer(r.Context(), auth.Account(r.Context()), id, playerID, r.FormValue("up") == "1")
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
	t := view.T(r.Context())
	switch msg, ok := errorText(t, err); {
	case ok:
		c.renderBoard(w, r, id, msg)
	case errors.Is(err, service.ErrInvalid):
		c.renderBoard(w, r, id, t.ErrInput())
	case isNotFound(err):
		http.NotFound(w, r)
	default:
		serverError(w, err)
	}
}

// handleDartError is handleGameError for entering or correcting the dart d: why the
// field doesn't exist.
func (c *Controller) handleDartError(w http.ResponseWriter, r *http.Request, id int64, d throw.Dart, err error) {
	switch {
	case d.Number == dartboard.BullNumber && d.Segment == dartboard.Triple:
		c.renderBoard(w, r, id, view.T(r.Context()).ErrTripleBull())
	case errors.Is(err, dartboard.ErrInvalid):
		c.renderBoard(w, r, id, view.T(r.Context()).ErrNoSuchField())
	default:
		c.handleGameError(w, r, id, err)
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
		return throw.Dart{Segment: dartboard.Miss}, true
	}
	number, err1 := strconv.ParseUint(r.FormValue("number"), 10, 8)
	segment, err2 := dartboard.ParseSegment(r.FormValue("segment"))
	if err1 != nil || err2 != nil {
		return throw.Dart{}, false
	}
	d := throw.Dart{Number: uint8(number), Segment: segment}
	if d.Number == dartboard.BullNumber {
		// The keypad's bull key follows the single/double choice.
		switch segment {
		case dartboard.Single:
			d.Segment = dartboard.Bull
		case dartboard.Double:
			d.Segment = dartboard.Bullseye
		}
	}
	return d, true
}
