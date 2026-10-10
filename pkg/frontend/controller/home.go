package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/frontend/view"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func (c *Controller) home(w http.ResponseWriter, r *http.Request) {
	c.renderHome(w, r, http.StatusOK, game.Mode(r.URL.Query().Get("mode")), "")
}

// renderHome shows the start page, with the options of mode if one is selected.
func (c *Controller) renderHome(w http.ResponseWriter, r *http.Request, status int, mode game.Mode, errMsg string) {
	if mode != game.ModeX01 && mode != game.ModeAroundTheClock {
		mode = ""
	}

	players, err := c.svc.ListPlayers(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	games, err := c.svc.ListGames(r.Context(), 10)
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, status, view.Home(view.HomeModel{Players: players, Games: games, Mode: mode, Error: errMsg}))
}

func (c *Controller) createPlayer(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.CreatePlayer(r.Context(), r.FormValue("name"))
	if errors.Is(err, service.ErrInvalid) {
		c.renderHome(w, r, http.StatusBadRequest, "", "Bitte einen Namen eingeben.")
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, homeURL(r), http.StatusSeeOther)
}

// homeURL returns the start page the request came from, so a selected mode
// stays selected after adding a player.
func homeURL(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host || ref.Path != "/" || ref.RawQuery == "" {
		return "/"
	}
	return "/?" + ref.RawQuery
}

func (c *Controller) createGame(w http.ResponseWriter, r *http.Request) {
	g, err := gameFromForm(r)
	if err == nil {
		var v service.GameView
		v, err = c.svc.CreateGame(r.Context(), g)
		if err == nil {
			http.Redirect(w, r, fmt.Sprintf("/games/%d", v.Game.ID), http.StatusSeeOther)
			return
		}
	}

	if !errors.Is(err, service.ErrInvalid) {
		serverError(w, err)
		return
	}
	msg := "Das Spiel konnte nicht gestartet werden. Bitte die Einstellungen prüfen."
	if len(r.Form["player_ids"]) == 0 {
		msg = "Bitte mindestens einen Mitspieler auswählen."
	}
	c.renderHome(w, r, http.StatusBadRequest, game.Mode(r.FormValue("mode")), msg)
}

func gameFromForm(r *http.Request) (game.Game, error) {
	if err := r.ParseForm(); err != nil {
		return game.Game{}, fmt.Errorf("%w: %w", service.ErrInvalid, err)
	}

	g := game.Game{Mode: game.Mode(r.FormValue("mode"))}
	for _, s := range r.Form["player_ids"] {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return g, fmt.Errorf("%w: invalid player id", service.ErrInvalid)
		}
		g.PlayerIDs = append(g.PlayerIDs, id)
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
		g.X01 = &x01.Settings{StartScore: score, OutMode: out}
	case game.ModeAroundTheClock:
		g.AroundTheClock = &aroundtheclock.Settings{
			Bull:            r.FormValue("bull") != "",
			MultiplierSkips: r.FormValue("multiplier_skips") != "",
		}
	}
	return g, nil
}
