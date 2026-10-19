package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/sj14/darts-o-mat/server/auth"
	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/server/domain/tournament"
	"github.com/sj14/darts-o-mat/server/frontend/view"
	"github.com/sj14/darts-o-mat/server/service"
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

func (c *Controller) createGuest(w http.ResponseWriter, r *http.Request) {
	guest, err := c.svc.CreateGuest(r.Context(), auth.Account(r.Context()), r.FormValue("name"))
	if isHTMX(r) {
		// From the player selection: only that part is answered, the new guest ticked.
		c.renderPlayers(w, r, err, guest.ID, 0)
		return
	}
	if errors.Is(err, service.ErrGuestNameTaken) {
		c.renderHome(w, r, http.StatusBadRequest, "", view.T(r.Context()).ErrGuestNameTaken())
		return
	}
	if errors.Is(err, service.ErrInvalid) {
		c.renderHome(w, r, http.StatusBadRequest, "", view.T(r.Context()).ErrEnterName())
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	http.Redirect(w, r, homeURL(r), http.StatusSeeOther)
}

// deleteGuest deletes a guest player, unless they are in a running game or tournament.
func (c *Controller) deleteGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := c.svc.DeleteGuest(r.Context(), auth.Account(r.Context()), id)
	if isHTMX(r) && !isNotFound(err) {
		c.renderPlayers(w, r, err, 0, id)
		return
	}
	switch {
	case err == nil:
		http.Redirect(w, r, homeURL(r), http.StatusSeeOther)
	case isNotFound(err):
		http.NotFound(w, r)
	case errors.Is(err, service.ErrGuestBusy):
		c.renderHome(w, r, http.StatusConflict, "", view.T(r.Context()).ErrGuestBusy())
	default:
		serverError(w, err)
	}
}

// isHTMX reports whether htmx sent the request, which then wants a part of the page.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

// renderPlayers answers adding or deleting a guest from the player selection with that
// selection: who was ticked stays ticked, the new guest (if any) joins them, and why it
// failed (err) is told within it. The status is 200 even then, htmx doesn't swap errors.
func (c *Controller) renderPlayers(w http.ResponseWriter, r *http.Request, err error, added, deleted int64) {
	ctx := r.Context()
	t := view.T(ctx)
	m := view.PlayersModel{Selected: map[int64]bool{}}
	switch {
	case err == nil:
	case errors.Is(err, service.ErrGuestNameTaken):
		m.Error = t.ErrGuestNameTaken()
	case errors.Is(err, service.ErrGuestBusy):
		m.Error = t.ErrGuestBusy()
	case errors.Is(err, service.ErrInvalid):
		m.Error = t.ErrEnterName()
	default:
		serverError(w, err)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	for _, s := range r.Form["player_ids"] {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil && id != deleted {
			m.Selected[id] = true
		}
	}
	if added != 0 {
		m.Selected[added] = true
	}
	if m.Players, err = c.svc.Players(ctx, auth.Account(ctx)); err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.PlayerPicker(m))
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

// createGame starts a game, or a tournament of the game's settings if a format was chosen.
func (c *Controller) createGame(w http.ResponseWriter, r *http.Request) {
	g, invite, err := gameFromForm(r)
	format := tournament.Format(r.FormValue("format"))
	switch {
	case err != nil:
	case format != "":
		t := tournament.Tournament{Format: format, Settings: g.Settings, Lobby: g.Lobby}
		for _, p := range g.Participants {
			t.Participants = append(t.Participants, tournament.Participant{PlayerID: p.PlayerID, BoardID: p.BoardID})
		}
		var v service.TournamentView
		if v, err = c.svc.CreateTournament(r.Context(), auth.Account(r.Context()), t, invite); err == nil {
			http.Redirect(w, r, tournamentURL(v.Tournament.ID), http.StatusSeeOther)
			return
		}
	default:
		var v service.GameView
		if v, err = c.svc.CreateGame(r.Context(), auth.Account(r.Context()), g, invite); err == nil {
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
	var unknown service.UnknownUserError
	var notFriend service.NotFriendError
	switch {
	case len(g.Participants) == 0:
		msg = texts.ErrChoosePlayer()
	case errors.Is(err, tournament.ErrPlayerCount):
		msg = texts.ErrTournamentPlayers(tournament.MinPlayers, tournament.MaxPlayers)
	case errors.As(err, &unknown):
		msg = texts.ErrUnknownUser(unknown.Username)
	case errors.As(err, &notFriend):
		msg = texts.ErrNotFriend(notFriend.Username)
	case errors.Is(err, service.ErrInviteSelf):
		msg = texts.ErrInviteSelf()
	}
	c.renderHome(w, r, http.StatusBadRequest, game.Mode(r.FormValue("mode")), msg)
}

// gameFromForm reads the new game and the usernames to invite from the form.
func gameFromForm(r *http.Request) (game.Game, []string, error) {
	if err := r.ParseForm(); err != nil {
		return game.Game{}, nil, fmt.Errorf("%w: %w", service.ErrInvalid, err)
	}

	g := game.Game{Settings: game.Settings{Mode: game.Mode(r.FormValue("mode"))}}
	for _, s := range r.Form["player_ids"] {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid player id", service.ErrInvalid)
		}
		g.Participants = append(g.Participants, game.Participant{PlayerID: id})
	}

	invite := r.Form["invite"]
	g.BullOff = r.FormValue("bull_off") != ""
	if r.FormValue("open") != "" {
		seats, err := strconv.Atoi(r.FormValue("seats"))
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid number of players", service.ErrInvalid)
		}
		g.Seats = seats
	}

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

	if s := r.FormValue("turn_seconds"); s != "" {
		seconds, err := strconv.Atoi(s)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid time per turn", service.ErrInvalid)
		}
		g.TurnSeconds = seconds
	}

	if s := r.FormValue("board_id"); s != "" {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return g, nil, fmt.Errorf("%w: invalid board id", service.ErrInvalid)
		}
		for i := range g.Participants {
			g.Participants[i].BoardID = &id
		}
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
		g.Rules = &x01.Settings{StartScore: score, OutMode: out}
	case game.ModeAroundTheClock:
		g.Rules = &aroundtheclock.Settings{
			Bull:            r.FormValue("bull") != "",
			MultiplierSkips: r.FormValue("multiplier_skips") != "",
		}
	}
	return g, invite, nil
}
