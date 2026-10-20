// Package api provides the JSON API. Except for register and login,
// all endpoints need the session cookie set by login.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

type API struct {
	svc *service.Service
}

func New(svc *service.Service) *API {
	return &API{svc: svc}
}

func (a *API) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/register", a.register)
	r.Post("/login", a.login)
	r.Post("/logout", a.logout)

	// Endpoints for a board's cameras (darts-o-cam), authenticated by the board's token.
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireBoard(a.svc, func(w http.ResponseWriter, r *http.Request) {
			writeError(w, service.ErrUnauthenticated)
		}))
		r.Post("/board/darts", a.addBoardDart)
		r.Post("/board/takeout", a.boardTakeout)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Require(a.svc, func(w http.ResponseWriter, r *http.Request) {
			writeError(w, service.ErrUnauthenticated)
		}))

		r.Get("/players", a.listPlayers)
		r.Post("/players", a.createGuest)
		r.Delete("/players/{id}", a.deleteGuest)
		r.Put("/account/email", a.setEmail)
		r.Delete("/account", a.deleteAccount)
		r.Get("/boards", a.listBoards)
		r.Post("/boards", a.createBoard)
		r.Delete("/boards/{id}", a.deleteBoard)
		r.Post("/boards/{id}/token", a.createBoardToken)

		r.Get("/games", a.listGames)
		r.Post("/games", a.createGame)
		r.Get("/games/{id}", a.getGame)
		r.Post("/games/{id}/darts", a.addDart)
		r.Delete("/games/{id}/darts/last", a.undoDart)
		r.Put("/games/{id}/darts/{dartID}", a.editDart)
		r.Post("/games/{id}/takeout", a.takeout)
		r.Post("/games/{id}/ratings", a.ratePlayer)
		r.Post("/games/{id}/leave", a.leaveGame)
		r.Post("/games/{id}/board", a.useBoard)
		r.Post("/games/{id}/bull-off", a.decideBullOff)

		r.Get("/lobbies", a.listLobbies)
		r.Post("/games/{id}/join", a.joinGame)
		r.Post("/tournaments/{id}/join", a.joinTournament)
		r.Get("/tournaments", a.listTournaments)
		r.Post("/tournaments", a.createTournament)
		r.Get("/tournaments/{id}", a.getTournament)
		r.Post("/tournaments/{id}/matches/{match}", a.startMatch)
		r.Post("/tournaments/{id}/accept", a.acceptTournament)
		r.Post("/tournaments/{id}/decline", a.declineTournament)
		r.Post("/tournaments/{id}/leave", a.leaveTournament)
		r.Get("/stats", a.stats)
		r.Get("/friends", a.listFriends)
		r.Post("/friends", a.requestFriend)
		r.Post("/friends/{accountID}/accept", a.acceptFriend)
		r.Delete("/friends/{accountID}", a.removeFriend)

		r.Get("/invitations", a.listInvitations)
		r.Post("/games/{id}/accept", a.acceptInvitation)
		r.Post("/games/{id}/decline", a.declineInvitation)
		r.Delete("/games/{id}/invitations/{playerID}", a.withdrawInvitation)
	})
	return r
}

func (a *API) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		// CaptchaToken is the answer of the Turnstile widget, needed if the server checks.
		CaptchaToken string `json:"captcha_token"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.svc.CheckCaptcha(r.Context(), req.CaptchaToken); err != nil {
		writeError(w, err)
		return
	}
	acc, err := a.svc.Register(r.Context(), req.Username, req.Email, req.Name, req.Password)
	respond(w, http.StatusCreated, acc, err)
}

func (a *API) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	acc, err := auth.Login(w, r, a.svc, req.Username, req.Password)
	respond(w, http.StatusOK, acc, err)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	if err := auth.Logout(w, r, a.svc); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listPlayers(w http.ResponseWriter, r *http.Request) {
	players, err := a.svc.Players(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, players, err)
}

func (a *API) createGuest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	p, err := a.svc.CreateGuest(r.Context(), auth.Account(r.Context()), req.Name)
	respond(w, http.StatusCreated, p, err)
}

// deleteGuest deletes a guest player of the account, unless they are in a running game or
// tournament; the games they played stay, with them shown as a deleted player.
func (a *API) deleteGuest(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	err := a.svc.DeleteGuest(r.Context(), auth.Account(r.Context()), id)
	respondEmpty(w, err)
}

// setEmail changes the email of the account ({"email": …}, empty removes it) and returns
// the account.
func (a *API) setEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if !decode(w, r, &req) {
		return
	}
	acc, err := a.svc.SetEmail(r.Context(), auth.Account(r.Context()), req.Email)
	respond(w, http.StatusOK, acc, err)
}

// deleteAccount deletes the account after checking its password (JSON {"password": …}),
// see service.DeleteAccount, and logs out.
func (a *API) deleteAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	err := a.svc.DeleteAccount(r.Context(), auth.Account(r.Context()), req.Password)
	if err == nil {
		err = auth.Logout(w, r, a.svc)
	}
	respondEmpty(w, err)
}

func (a *API) listBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := a.svc.Boards(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, boards, err)
}

func (a *API) createBoard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &req) {
		return
	}
	b, err := a.svc.CreateBoard(r.Context(), auth.Account(r.Context()), req.Name)
	respond(w, http.StatusCreated, b, err)
}

func (a *API) deleteBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := a.svc.DeleteBoard(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) createBoardToken(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	token, err := a.svc.CreateBoardToken(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusCreated, map[string]string{"token": token}, err)
}

// addBoardDart enters a dart detected by the board's cameras into the running game on the board.
func (a *API) addBoardDart(w http.ResponseWriter, r *http.Request) {
	var h dartboard.Hit // only what the cameras know, no player or takeout
	if !decode(w, r, &h) {
		return
	}
	v, err := a.svc.AddBoardDart(r.Context(), auth.Board(r.Context()), throw.FromHit(h))
	respond(w, http.StatusCreated, v, err)
}

// boardTakeout reports that the darts were pulled out of the board.
func (a *API) boardTakeout(w http.ResponseWriter, r *http.Request) {
	v, err := a.svc.BoardTakeout(r.Context(), auth.Board(r.Context()))
	respond(w, http.StatusOK, v, err)
}

func (a *API) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := a.svc.ListGames(r.Context(), auth.Account(r.Context()), 50)
	respond(w, http.StatusOK, games, err)
}

// createRequest is the body to create a game or a tournament.
type createRequest struct {
	// Format is the tournament's format; a game has none.
	Format tournament.Format `json:"format,omitempty"`
	game.Settings
	game.Lobby
	// PlayerIDs are the account's own player and its guests, in throwing order.
	PlayerIDs []int64 `json:"player_ids"`
	// BoardID is the board these players throw on, one of the account's.
	BoardID *int64 `json:"board_id"`
	// Invite lists usernames of other accounts to invite; they are added after PlayerIDs.
	Invite []string `json:"invite"`
}

func (a *API) createGame(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decodeGame(w, r, &req, &req.Settings) {
		return
	}
	g := game.Game{Settings: req.Settings, Lobby: req.Lobby}
	for _, id := range req.PlayerIDs {
		g.Participants = append(g.Participants, game.Participant{PlayerID: id, BoardID: req.BoardID})
	}
	v, err := a.svc.CreateGame(r.Context(), auth.Account(r.Context()), g, req.Invite)
	respond(w, http.StatusCreated, v, err)
}

func (a *API) getGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.Game(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

func (a *API) addDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	var d throw.Dart
	if !decode(w, r, &d) {
		return
	}
	v, err := a.svc.AddDart(r.Context(), auth.Account(r.Context()), id, d)
	respond(w, http.StatusCreated, v, err)
}

// editDart corrects what an earlier dart hit.
func (a *API) editDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	dartID, ok := urlID(w, r, "dartID")
	if !ok {
		return
	}
	var d throw.Dart
	if !decode(w, r, &d) {
		return
	}
	v, err := a.svc.EditDart(r.Context(), auth.Account(r.Context()), id, dartID, d)
	respond(w, http.StatusOK, v, err)
}

func (a *API) undoDart(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.UndoDart(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

// stats returns the statistics of the account's player and its guests over all finished games.
func (a *API) stats(w http.ResponseWriter, r *http.Request) {
	summaries, err := a.svc.Stats(r.Context(), auth.Account(r.Context()), "")
	respond(w, http.StatusOK, summaries, err)
}

func (a *API) listFriends(w http.ResponseWriter, r *http.Request) {
	friends, err := a.svc.Friends(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, friends, err)
}

// requestFriend sends a friend request, or accepts the request the other account already sent.
func (a *API) requestFriend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := a.svc.RequestFriend(r.Context(), auth.Account(r.Context()), req.Username); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) acceptFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "accountID")
	if !ok {
		return
	}
	if err := a.svc.AcceptFriend(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// removeFriend ends a friendship, or declines or withdraws a request.
func (a *API) removeFriend(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "accountID")
	if !ok {
		return
	}
	if err := a.svc.RemoveFriend(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// takeout confirms that the darts were pulled out, so the next player throws.
func (a *API) takeout(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.Takeout(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

// ratePlayer gives another player of a finished game a thumb: {"player_id": 2, "up": true}.
func (a *API) ratePlayer(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		PlayerID int64 `json:"player_id"`
		Up       bool  `json:"up"`
	}
	if !decode(w, r, &req) {
		return
	}
	v, err := a.svc.RatePlayer(r.Context(), auth.Account(r.Context()), id, req.PlayerID, req.Up)
	respond(w, http.StatusOK, v, err)
}

// decideBullOff settles a tie when bulling off: {"player_id": 2} was closer.
func (a *API) decideBullOff(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		PlayerID int64 `json:"player_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	v, err := a.svc.DecideBullOff(r.Context(), auth.Account(r.Context()), id, req.PlayerID)
	respond(w, http.StatusOK, v, err)
}

// leaveGame lets the account leave the game with all its players: in the lobby they are
// removed, in a running game the others play on without them, or it is aborted if fewer
// than two players would be left (which is all that "abort" is in a game of one account).
func (a *API) leaveGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.LeaveGame(r.Context(), auth.Account(r.Context()), id)
	if err == nil && v.Game.ID == 0 { // left the lobby: nothing to show anymore
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respond(w, http.StatusOK, v, err)
}

// useBoard puts the account's players of a running game on a board; a game that
// was on the board continues without it. A null board_id takes the players off their
// board: they enter their darts by hand from then on.
func (a *API) useBoard(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	var req struct {
		BoardID *int64 `json:"board_id"`
	}
	if !decode(w, r, &req) {
		return
	}
	v, err := a.svc.UseBoard(r.Context(), auth.Account(r.Context()), id, req.BoardID)
	respond(w, http.StatusOK, v, err)
}

func (a *API) listInvitations(w http.ResponseWriter, r *http.Request) {
	games, err := a.svc.Invitations(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, games, err)
}

func (a *API) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, ok := optionalBoard(w, r)
	if !ok {
		return
	}
	v, err := a.svc.AcceptInvitation(r.Context(), auth.Account(r.Context()), id, boardID)
	respond(w, http.StatusOK, v, err)
}

func (a *API) declineInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	if err := a.svc.DeclineInvitation(r.Context(), auth.Account(r.Context()), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) withdrawInvitation(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	playerID, ok := urlID(w, r, "playerID")
	if !ok {
		return
	}
	v, err := a.svc.WithdrawInvitation(r.Context(), auth.Account(r.Context()), id, playerID)
	respond(w, http.StatusOK, v, err)
}

// urlID parses a numeric URL parameter and writes the error response if that fails.
func urlID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, fmt.Errorf("%w: invalid %s", service.ErrInvalid, name))
		return 0, false
	}
	return id, true
}

// decode reads the JSON body into v and writes the error response if that fails.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return false
	}
	return true
}

// decodeGame decodes the request to create a game or tournament into v, which has the
// settings s: the rules in it are those of the mode, so the mode is read first and the
// rules prepared for it (see game.Settings.Rules).
func decodeGame(w http.ResponseWriter, r *http.Request, v any, s *game.Settings) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return false
	}
	var head struct {
		Mode game.Mode `json:"mode"`
	}
	if err := json.Unmarshal(body, &head); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return false
	}
	s.Rules = game.NewRules(head.Mode)
	if err := json.Unmarshal(body, v); err != nil {
		writeError(w, fmt.Errorf("%w: %w", service.ErrInvalid, err))
		return false
	}
	return true
}

func (a *API) listTournaments(w http.ResponseWriter, r *http.Request) {
	ts, err := a.svc.ListTournaments(r.Context(), auth.Account(r.Context()), 50)
	respond(w, http.StatusOK, ts, err)
}

func (a *API) createTournament(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if !decodeGame(w, r, &req, &req.Settings) {
		return
	}
	t := tournament.Tournament{Format: req.Format, Settings: req.Settings, Lobby: req.Lobby}
	for _, id := range req.PlayerIDs {
		t.Participants = append(t.Participants, tournament.Participant{PlayerID: id, BoardID: req.BoardID})
	}
	v, err := a.svc.CreateTournament(r.Context(), auth.Account(r.Context()), t, req.Invite)
	respond(w, http.StatusCreated, v, err)
}

func (a *API) getTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.Tournament(r.Context(), auth.Account(r.Context()), id)
	respond(w, http.StatusOK, v, err)
}

// startMatch starts the game of a match; the answer is that game.
func (a *API) startMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	match, ok := urlID(w, r, "match")
	if !ok {
		return
	}
	v, err := a.svc.StartMatch(r.Context(), auth.Account(r.Context()), id, int(match))
	respond(w, http.StatusCreated, v, err)
}

func (a *API) acceptTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, ok := optionalBoard(w, r)
	if !ok {
		return
	}
	v, err := a.svc.AcceptTournamentInvitation(r.Context(), auth.Account(r.Context()), id, boardID)
	respond(w, http.StatusOK, v, err)
}

func (a *API) declineTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	respondEmpty(w, a.svc.DeclineTournamentInvitation(r.Context(), auth.Account(r.Context()), id))
}

func (a *API) leaveTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	v, err := a.svc.LeaveTournament(r.Context(), auth.Account(r.Context()), id)
	if err == nil && v.Tournament.ID == 0 { // left the lobby: nothing to show anymore
		w.WriteHeader(http.StatusNoContent)
		return
	}
	respond(w, http.StatusOK, v, err)
}

// optionalBoard reads the optional body {"board_id": 1}: the board the player throws on.
func optionalBoard(w http.ResponseWriter, r *http.Request) (*int64, bool) {
	var req struct {
		BoardID *int64 `json:"board_id"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return nil, false
	}
	return req.BoardID, true
}

// listLobbies lists the games and tournaments of others that are open for strangers,
// narrowed down by the filter of the query (see service.LobbyFilterFromQuery).
func (a *API) listLobbies(w http.ResponseWriter, r *http.Request) {
	lobbies, err := a.svc.OpenLobbies(r.Context(), auth.Account(r.Context()))
	respond(w, http.StatusOK, service.LobbyFilterFromQuery(r.URL.Query()).Apply(lobbies), err)
}

// joinGame takes a seat of an open game with the account's player.
func (a *API) joinGame(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, ok := optionalBoard(w, r)
	if !ok {
		return
	}
	v, err := a.svc.JoinGame(r.Context(), auth.Account(r.Context()), id, boardID)
	respond(w, http.StatusOK, v, err)
}

// joinTournament takes a seat of an open tournament with the account's player.
func (a *API) joinTournament(w http.ResponseWriter, r *http.Request) {
	id, ok := urlID(w, r, "id")
	if !ok {
		return
	}
	boardID, ok := optionalBoard(w, r)
	if !ok {
		return
	}
	v, err := a.svc.JoinTournament(r.Context(), auth.Account(r.Context()), id, boardID)
	respond(w, http.StatusOK, v, err)
}

// respondEmpty answers with 204 and no body, or with the error.
func respondEmpty(w http.ResponseWriter, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrAlreadyFriends):
		status = http.StatusConflict
	case errors.Is(err, service.ErrInvalid), errors.Is(err, dartboard.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, service.ErrTooManyLogins):
		status = http.StatusTooManyRequests
	case errors.Is(err, service.ErrUnauthenticated):
		status = http.StatusUnauthorized
	case errors.Is(err, service.ErrForbidden), errors.Is(err, service.ErrRegistrationClosed):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, turn.ErrGameOver), errors.Is(err, turn.ErrWaitingForTakeout), errors.Is(err, service.ErrNothingToUndo), errors.Is(err, service.ErrNoActiveGame), errors.Is(err, service.ErrNotOnBoard), errors.Is(err, service.ErrCantLeave),
		errors.Is(err, service.ErrNotReady), errors.Is(err, service.ErrAborted), errors.Is(err, service.ErrCompleted), errors.Is(err, service.ErrUsernameTaken), errors.Is(err, service.ErrEmailTaken):
		status = http.StatusConflict
	}

	msg := err.Error()
	if status == http.StatusInternalServerError {
		log.Printf("api: %v", err)
		msg = http.StatusText(status)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}
