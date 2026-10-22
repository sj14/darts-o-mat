package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

func (c *Controller) home(w http.ResponseWriter, r *http.Request) {
	c.renderHome(w, r, http.StatusOK, game.Mode(r.URL.Query().Get("mode")), view.Alert{})
}

// renderHome shows the start page, with the options of mode if one is selected.
func (c *Controller) renderHome(w http.ResponseWriter, r *http.Request, status int, mode game.Mode, failed view.Alert) {
	if mode != game.ModeX01 && mode != game.ModeAroundTheClock {
		mode = ""
	}

	ctx := r.Context()
	m := view.HomeModel{Account: auth.Account(ctx), Mode: mode, Error: failed}
	var err error
	if m.Players, err = c.svc.Players(ctx, m.Account); err != nil {
		serverError(w, err)
		return
	}
	if m.Boards, err = c.svc.Boards(ctx, m.Account); err != nil {
		serverError(w, err)
		return
	}
	if m.Games, err = c.svc.ListGames(ctx, m.Account, 10); err != nil {
		serverError(w, err)
		return
	}
	if m.Invitations, err = c.svc.Invitations(ctx, m.Account); err != nil {
		serverError(w, err)
		return
	}
	if m.Friends, err = c.svc.Friends(ctx, m.Account); err != nil {
		serverError(w, err)
		return
	}
	if m.Tournaments, err = c.svc.ListTournaments(ctx, m.Account, 5); err != nil {
		serverError(w, err)
		return
	}
	if m.TournamentInvitations, err = c.svc.TournamentInvitations(ctx, m.Account); err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Home(m))
}

// deleteGuest deletes a guest player from the settings, unless they are in a running game
// or tournament.
func (c *Controller) deleteGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := c.svc.DeleteGuest(r.Context(), auth.Account(r.Context()), id)
	if isNotFound(err) {
		http.NotFound(w, r)
		return
	}
	c.respondSettings(w, r, err)
}

// isHTMX reports whether htmx sent the request, which then wants a part of the page.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

// createGame creates a game in its lobby, or a tournament of the game's settings if a
// format was chosen, and goes to it: players are added and invited there.
func (c *Controller) createGame(w http.ResponseWriter, r *http.Request) {
	g, err := gameFromForm(r)
	var boardID *int64
	if err == nil {
		boardID, err = boardFromForm(r)
	}
	playerID, _ := strconv.ParseInt(r.FormValue("player_id"), 10, 64) // empty: the own player
	format := tournament.Format(r.FormValue("format"))
	switch {
	case err != nil:
	case format != "":
		var v service.TournamentView
		t := tournament.Tournament{Format: format, Settings: g.Settings, Lobby: g.Lobby}
		if v, err = c.svc.CreateTournament(r.Context(), auth.Account(r.Context()), t, playerID, boardID); err == nil {
			http.Redirect(w, r, tournamentURL(v.Tournament.ID), http.StatusSeeOther)
			return
		}
	default:
		var v service.GameView
		if v, err = c.svc.CreateGame(r.Context(), auth.Account(r.Context()), g, playerID, boardID); err == nil {
			http.Redirect(w, r, gameURL(v.Game.ID), http.StatusSeeOther)
			return
		}
	}

	if !errors.Is(err, service.ErrInvalid) {
		serverError(w, err)
		return
	}
	texts := view.T(r.Context())
	msg := texts.ErrGameNotStarted()
	if text, ok := errorText(texts, err); ok {
		msg = text
	}
	c.renderHome(w, r, http.StatusBadRequest, game.Mode(r.FormValue("mode")), alertOf(err, msg))
}

// gameFromForm reads the settings of the new game from the form.
func gameFromForm(r *http.Request) (game.Game, error) {
	if err := r.ParseForm(); err != nil {
		return game.Game{}, fmt.Errorf("%w: %w", service.ErrInvalid, err)
	}

	g := game.Game{Settings: game.Settings{Mode: game.Mode(r.FormValue("mode"))}}
	g.BullOff = r.FormValue("bull_off") != ""
	if r.FormValue("open") != "" {
		seats, err := strconv.Atoi(r.FormValue("seats"))
		if err != nil {
			return g, fmt.Errorf("%w: invalid number of players", service.ErrInvalid)
		}
		g.Seats = seats
		g.BoardOnly = r.FormValue("board_only") != ""
	}

	if s := r.FormValue("legs"); s != "" {
		legs, err := strconv.Atoi(s)
		if err != nil {
			return g, fmt.Errorf("%w: invalid legs", service.ErrInvalid)
		}
		g.Legs = legs
	}
	if s := r.FormValue("sets"); s != "" {
		sets, err := strconv.Atoi(s)
		if err != nil {
			return g, fmt.Errorf("%w: invalid sets", service.ErrInvalid)
		}
		g.Sets = sets
	}

	if s := r.FormValue("turn_seconds"); s != "" {
		seconds, err := strconv.Atoi(s)
		if err != nil {
			return g, fmt.Errorf("%w: invalid time per turn", service.ErrInvalid)
		}
		g.TurnSeconds = seconds
	}

	switch g.Mode {
	case game.ModeX01:
		score, err := strconv.ParseUint(r.FormValue("start_score"), 10, 64)
		if err != nil {
			return g, fmt.Errorf("%w: invalid start score", service.ErrInvalid)
		}
		var out x01.OutMode
		if err := out.UnmarshalText([]byte(r.FormValue("out_mode"))); err != nil {
			return g, fmt.Errorf("%w: %w", service.ErrInvalid, err)
		}
		g.Rules = &x01.Settings{StartScore: score, OutMode: out}
	case game.ModeAroundTheClock:
		g.Rules = &aroundtheclock.Settings{
			Bull:            r.FormValue("bull") != "",
			MultiplierSkips: r.FormValue("multiplier_skips") != "",
		}
	}
	return g, nil
}
