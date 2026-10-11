package controller

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

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
	flusher, ok := w.(http.Flusher)
	if !ok {
		serverError(w, errors.New("streaming not supported"))
		return
	}

	changes, stop := c.svc.Subscribe(id)
	defer stop()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // don't let a reverse proxy hold events back
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Comments keep idle connections from being closed by proxies.
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
		case <-changes:
			v, err := c.svc.Game(r.Context(), acc, id)
			if err != nil {
				return // e.g. the game was deleted because everyone declined
			}
			var buf bytes.Buffer
			if err := view.Board(v, "").Render(r.Context(), &buf); err != nil {
				log.Printf("render board: %v", err)
				return
			}
			writeEvent(w, "board", buf.String())
		}
		flusher.Flush()
	}
}

// writeEvent writes one Server-Sent Event; every line of the data needs its own "data:" field.
func writeEvent(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}

// addDart is called by htmx and responds with the updated board.
func (c *Controller) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}

	d, ok := dartFromForm(r)
	if !ok {
		c.renderBoard(w, r, id, "Ungültige Eingabe.")
		return
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
	i := slices.IndexFunc(v.Darts, func(d throw.Dart) bool { return d.ID == dartID && !d.Takeout })
	if i < 0 {
		http.NotFound(w, r)
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
		c.renderBoard(w, r, id, "Ungültige Eingabe.")
		return
	}
	v, err := c.svc.EditDart(r.Context(), auth.Account(r.Context()), id, dartID, d)
	switch {
	case err == nil:
		render(w, r, http.StatusOK, view.Board(v, ""))
	case errors.Is(err, service.ErrChangesThrowers):
		c.renderBoard(w, r, id, "Diese Korrektur würde ändern, wer die folgenden Darts geworfen hat (z. B. Überworfen mitten in der Aufnahme). Nimm stattdessen die späteren Darts zurück und trage sie neu ein.")
	case errors.Is(err, turn.ErrGameOver):
		c.renderBoard(w, r, id, "Mit dieser Korrektur wäre das Spiel schon vor späteren Darts vorbei. Nimm stattdessen die späteren Darts zurück.")
	case d.Number == throw.Bull && d.Multiplier == 3:
		c.renderBoard(w, r, id, "Es gibt kein Triple-Bull.")
	case errors.Is(err, throw.ErrInvalid):
		c.renderBoard(w, r, id, "Diesen Treffer gibt es nicht.")
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
		c.renderBoard(w, r, id, "Gerade ist beim Ausbullen niemand gleichauf.")
	default:
		c.handleGameError(w, r, id, err)
	}
}

// abortGame is called by htmx when the owner aborts the game.
func (c *Controller) abortGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := c.svc.AbortGame(r.Context(), auth.Account(r.Context()), id)
	c.respondBoard(w, r, id, v, err)
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
		c.renderBoard(w, r, id, "Das Spiel ist schon vorbei.")
	case errors.Is(err, turn.ErrWaitingForTakeout):
		c.renderBoard(w, r, id, "Erst die Pfeile ziehen, dann ist der Nächste dran.")
	case errors.Is(err, service.ErrNotReady):
		c.renderBoard(w, r, id, "Es haben noch nicht alle Eingeladenen zugesagt.")
	case errors.Is(err, service.ErrAborted):
		c.renderBoard(w, r, id, "Das Spiel wurde abgebrochen.")
	case errors.Is(err, service.ErrNothingToUndo):
		c.renderBoard(w, r, id, "Es gibt keinen Dart zum Zurücknehmen.")
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

// dartFromForm reads the dart entered with the keypad.
func dartFromForm(r *http.Request) (throw.Dart, bool) {
	d := throw.Dart{Miss: r.FormValue("miss") == "true"}
	if d.Miss {
		return d, true
	}
	number, err1 := strconv.ParseUint(r.FormValue("number"), 10, 8)
	multiplier, err2 := strconv.ParseUint(r.FormValue("multiplier"), 10, 8)
	if err1 != nil || err2 != nil {
		return d, false
	}
	d.Number, d.Multiplier = uint8(number), uint8(multiplier)
	return d, true
}
