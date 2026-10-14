package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sj14/darts-o-mat/pkg/auth"
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

	ctx := r.Context()
	m := view.HomeModel{Account: auth.Account(ctx), Mode: mode, Error: errMsg}
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
	render(w, r, status, view.Home(m))
}

func (c *Controller) createGuest(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.CreateGuest(r.Context(), auth.Account(r.Context()), r.FormValue("name"))
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
// stays selected after adding a guest.
func homeURL(r *http.Request) string {
	ref, err := url.Parse(r.Referer())
	if err != nil || ref.Host != r.Host || ref.Path != "/" || ref.RawQuery == "" {
		return "/"
	}
	return "/?" + ref.RawQuery
}

func (c *Controller) createGame(w http.ResponseWriter, r *http.Request) {
	g, invite, err := gameFromForm(r)
	if err == nil {
		var v service.GameView
		v, err = c.svc.CreateGame(r.Context(), auth.Account(r.Context()), g, invite)
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
	var unknown service.UnknownUserError
	var notFriend service.NotFriendError
	switch {
	case len(g.PlayerIDs) == 0 && len(invite) == 0:
		msg = "Bitte mindestens einen Mitspieler auswählen oder einladen."
	case errors.As(err, &unknown):
		msg = fmt.Sprintf("Einen Account mit dem Benutzernamen „%s“ gibt es nicht.", unknown.Username)
	case errors.As(err, &notFriend):
		msg = fmt.Sprintf("„%s“ ist nicht mit dir befreundet und kann deshalb nicht eingeladen werden.", notFriend.Username)
	case errors.Is(err, service.ErrInviteSelf):
		msg = "Du kannst dich nicht selbst einladen. Kreuze dich stattdessen bei den Mitspielern an."
	}
	c.renderHome(w, r, http.StatusBadRequest, game.Mode(r.FormValue("mode")), msg)
}

// gameFromForm reads the new game and the usernames to invite from the form.
func gameFromForm(r *http.Request) (game.Game, []string, error) {
	if err := r.ParseForm(); err != nil {
		return game.Game{}, nil, fmt.Errorf("%w: %w", service.ErrInvalid, err)
	}

	g := game.Game{Mode: game.Mode(r.FormValue("mode"))}
	for _, s := range r.Form["player_ids"] {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid player id", service.ErrInvalid)
		}
		g.PlayerIDs = append(g.PlayerIDs, id)
	}

	invite := r.Form["invite"]
	g.BullOff = r.FormValue("bull_off") != ""

	if s := r.FormValue("legs"); s != "" {
		legs, err := strconv.Atoi(s)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid legs", service.ErrInvalid)
		}
		g.Legs = legs
	}
	if s := r.FormValue("sets"); s != "" {
		sets, err := strconv.Atoi(s)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid sets", service.ErrInvalid)
		}
		g.Sets = sets
	}

	if s := r.FormValue("board_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid board id", service.ErrInvalid)
		}
		g.BoardID = &id
	}

	switch g.Mode {
	case game.ModeX01:
		score, err := strconv.ParseUint(r.FormValue("start_score"), 10, 64)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid start score", service.ErrInvalid)
		}
		var out x01.OutMode
		if err := out.UnmarshalText([]byte(r.FormValue("out_mode"))); err != nil {
			return g, nil, fmt.Errorf("%w: %w", service.ErrInvalid, err)
		}
		g.X01 = &x01.Settings{StartScore: score, OutMode: out}
	case game.ModeAroundTheClock:
		g.AroundTheClock = &aroundtheclock.Settings{
			Bull:            r.FormValue("bull") != "",
			MultiplierSkips: r.FormValue("multiplier_skips") != "",
		}
	}
	return g, invite, nil
}
