// Package service contains the use cases shared by the JSON API and the web frontend.
// All game operations are done on behalf of a logged-in account and check its permissions.
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/notify"
	"github.com/sj14/darts-o-mat/internal/server/captcha"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

var (
	// ErrInvalid wraps errors caused by invalid input, e.g. unknown players or wrong settings.
	ErrInvalid = errors.New("invalid input")
	// ErrNotFound is also returned for games the account takes no part in, so their existence isn't revealed.
	ErrNotFound      = store.ErrNotFound
	ErrForbidden     = errors.New("not allowed")
	ErrNotReady      = errors.New("waiting for invited players to accept")
	ErrNothingToUndo = errors.New("no dart to undo")
	ErrAborted       = errors.New("the game was aborted")
	// ErrCompleted means the result of the game is final, see game.Game.Completed.
	ErrCompleted = errors.New("the game is completed, its result can't be changed anymore")
	// ErrNotOver means the game isn't over yet, e.g. to rate its players.
	ErrNotOver = fmt.Errorf("%w: the game isn't over", ErrInvalid)
	// ErrChangesThrowers means a correction would change who threw later darts,
	// e.g. a bust that ends a turn early: the later darts have to be taken back instead.
	ErrChangesThrowers = fmt.Errorf("%w: the correction would change who threw the later darts", ErrInvalid)
	ErrInviteSelf      = fmt.Errorf("%w: you can't invite yourself", ErrInvalid)
	// ErrGuestNameTaken means the account already has a player (itself or a guest) with that name.
	ErrGuestNameTaken = fmt.Errorf("%w: a player with this name already exists", ErrInvalid)
)

// NotFriendError is returned when inviting an account that isn't a friend. It wraps ErrInvalid.
type NotFriendError struct {
	Username string
}

func (e NotFriendError) Error() string {
	return fmt.Sprintf("%v: %q is not your friend", ErrInvalid, e.Username)
}

func (e NotFriendError) Unwrap() error {
	return ErrInvalid
}

// UnknownUserError is returned when inviting a username that doesn't exist. It wraps ErrInvalid.
type UnknownUserError struct {
	Username string
}

func (e UnknownUserError) Error() string {
	return fmt.Sprintf("%v: unknown user %q", ErrInvalid, e.Username)
}

func (e UnknownUserError) Unwrap() error {
	return ErrInvalid
}

type Options struct {
	// Registration allows anyone to create an account.
	Registration bool
	// MaxLoginFailures is how many failed logins a username may have within 15 minutes
	// before logging in as it is refused (ErrTooManyLogins) until they are older; 0 is no
	// limit.
	MaxLoginFailures int
	// ClientIPHeader names the header in which a proxy in front of the server passes the
	// address of the client, e.g. X-Forwarded-For; empty uses the address of the connection.
	// Only set it if all requests come through that proxy, anyone can send the header
	// otherwise. See Service.ClientNetwork.
	ClientIPHeader string
	// Captcha checks that registrations come from a person; nil checks nothing.
	Captcha captcha.Verifier
}

type Service struct {
	store *store.Store
	opts  Options
	// mu serializes changes to the darts and players of games, so two darts
	// can't both pass the replay check against the same previous state.
	mu sync.Mutex
	// games notifies about changed games, by ID.
	games notify.Hub[int64]
	// tournaments notifies about changed tournaments, by ID: their own changes and those
	// of their games.
	tournaments notify.Hub[int64]
	// timers end the turn of a game with a time limit, by game ID (see watchTurn).
	// logins limits the failed logins per username, nil without a limit.
	logins   *loginLimiter
	networks *loginLimiter
	timers   map[int64]*time.Timer
	timersMu sync.Mutex
	// now is the time, replaced in tests.
	now func() time.Time
	// started is when the server started: no turn ends earlier than its time after it
	// (see GameView.TurnDeadline).
	started time.Time
	// pages are the texts of the pages, read once (see Pages); SetPage replaces the map.
	pages   map[page.Name]string
	pagesMu sync.Mutex
}

func New(store *store.Store, opts Options) *Service {
	s := &Service{store: store, opts: opts, timers: map[int64]*time.Timer{}, now: time.Now, started: time.Now()}
	s.limitLogins(opts.MaxLoginFailures)
	return s
}

func (s *Service) RegistrationOpen() bool {
	return s.opts.Registration
}

// GameView is a game together with everything needed to show it to an account.
type GameView struct {
	Game    game.Game       `json:"game"`
	Players []player.Player `json:"players"` // in throwing order
	Darts   []throw.Dart    `json:"darts"`   // the log: darts, takeouts and players leaving
	State   game.State      `json:"state"`   // x01.State or aroundtheclock.State, depending on the mode
	// Completed is set once the result is final, see game.Game.Completed. Darts can't be
	// undone or corrected anymore then.
	Completed bool `json:"completed"`
	// Ready is true once no invitation is pending anymore and no seat is free; only then
	// darts can be thrown.
	Ready bool `json:"ready"`
	// BoardNames are the names of the boards the players throw on, by player.
	BoardNames map[int64]string `json:"board_names,omitempty"`
	// TurnDeadline is when the turn of the player who is up ends, nil without a limit
	// (game.Game.TurnDeadline). It is never earlier than the turn's time after the server
	// started (user's wish 2026-10-20): the time it was down for a restart or an update
	// doesn't cost anyone a turn.
	TurnDeadline *time.Time `json:"turn_deadline,omitempty"`

	// The following fields depend on the account the view was created for.
	CanThrow bool `json:"can_throw"` // may enter and undo darts (once the game is ready, never after it was aborted)
	Invited  bool `json:"invited"`   // has a pending invitation to answer
	// HasLeft is set if the account's players left the game (LeaveGame); it can look at it
	// but not throw anymore.
	HasLeft bool `json:"has_left,omitempty"`
	// Mine are the players of the account the view was created for that take part: its
	// own player and its guests.
	Mine []int64 `json:"-"`
	// OwnPlayerID is the account's own player if it takes part in the game, else 0.
	OwnPlayerID int64 `json:"-"`
	// Ratings are the thumbs the account gave in this game, by rated player (true is up);
	// only once the game is over.
	Ratings map[int64]bool `json:"ratings,omitempty"`
	// Boards are the boards the viewer can pick: its own and those the other players of
	// the game throw on (friends at one board share it). Only for an invited player, to
	// accept on one, and for a viewer who can still throw, to change the board or take
	// their players off it (UseBoard).
	Boards []BoardChoice `json:"boards,omitempty"`
}

// MayEdit reports whether the viewer may correct the entry of the log at index i: it is a
// dart of one of their players.
func (v GameView) MayEdit(i int) bool {
	return !v.Completed && i >= 0 && i < len(v.Darts) && v.Darts[i].IsDart() && v.IsMine(v.Darts[i].PlayerID)
}

// MayUndo reports whether the viewer may take back the last entry of the log: the last
// dart, or the takeout after it, is one of their players'. A player leaving can't be
// taken back.
func (v GameView) MayUndo() bool {
	if v.Completed {
		return false
	}
	for i := len(v.Darts) - 1; i >= 0; i-- {
		if v.Darts[i].Leave || v.Darts[i].Skip {
			return false
		}
		if v.Darts[i].IsDart() {
			return v.MayEdit(i)
		}
	}
	return false
}

// IsMine reports whether the player is one of the viewer's.
func (v GameView) IsMine(playerID int64) bool {
	return slices.Contains(v.Mine, playerID)
}

// BoardOf returns the board a player throws on, nil if they enter their darts by hand.
func (v GameView) BoardOf(playerID int64) *int64 {
	if p, ok := v.Game.Participant(playerID); ok {
		return p.BoardID
	}
	return nil
}

// HasCamera reports whether a player throws on a board with cameras.
func (v GameView) HasCamera(playerID int64) bool {
	return v.BoardOf(playerID) != nil
}

// HasBoard reports whether any player of the game throws on a board.
func (v GameView) HasBoard() bool {
	return v.Game.WaitsForTakeout()
}

// PlaysOn reports whether the player who is up throws on the board.
func (v GameView) PlaysOn(boardID int64) bool {
	cur := v.Common().CurrentPlayerID
	if cur == nil {
		return false
	}
	b := v.BoardOf(*cur)
	return b != nil && *b == boardID
}

// OnlyMine reports whether all players of the game are the viewer's: a game of one account
// alone, with guests.
func (v GameView) OnlyMine() bool {
	return len(v.Mine) == len(v.Game.Participants)
}

// Finished reports whether nothing happens in the game anymore: it is over or was
// aborted. A game that isn't finished is running, or waiting in the lobby (Ready).
func (v GameView) Finished() bool {
	return v.Game.Aborted() || v.Common().GameOver
}

// Common returns the part of the game's state that is the same for all modes.
func (v GameView) Common() turn.State {
	if v.State == nil {
		return turn.State{}
	}
	return v.State.Common()
}

// Pending reports whether the player hasn't answered the invitation to the game yet.
func (v GameView) Pending(playerID int64) bool {
	p, ok := v.Game.Participant(playerID)
	return ok && p.Status == game.InvitationPending
}

// CreateGame creates a game for the account. Its Participants hold the account's own
// player and its guests, in throwing order, each with the board they throw on (the same
// for all, or none); friends are invited by username and added to the end of the
// throwing order. Nobody owns the game: the account has no more rights in it than the
// others.
func (s *Service) CreateGame(ctx context.Context, acc account.Account, g game.Game, inviteUsernames []string) (GameView, error) {
	g.ID = 0
	g.Settings = g.Settings.WithDefaults()

	if len(g.Participants) == 0 {
		return GameView{}, fmt.Errorf("%w: choose at least one of your players", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOwnPlayers(ctx, acc, g.PlayerIDs()); err != nil {
		return GameView{}, err
	}
	for i, p := range g.Participants {
		g.Participants[i].Status = game.InvitationAccepted
		if err := s.checkSeatBoard(ctx, acc, seat{lobby: g.Lobby, holdsBoards: true}, p.BoardID); err != nil {
			return GameView{}, err
		}
	}
	own := g.PlayerIDs()
	invited, err := s.invitedPlayers(ctx, acc, inviteUsernames)
	if err != nil {
		return GameView{}, err
	}
	for _, id := range invited {
		g.Participants = append(g.Participants, game.Participant{PlayerID: id, Status: game.InvitationPending})
	}

	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.checkPlayersFree(ctx, acc, own, 0); err != nil {
		return GameView{}, err
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return GameView{}, err
	}
	if err := s.store.CreateGame(ctx, &g); err != nil {
		return GameView{}, err
	}
	if err := s.takeBoards(ctx, g); err != nil {
		return GameView{}, err
	}
	if err := s.startGame(ctx, g.ID); err != nil {
		return GameView{}, err
	}
	return s.Game(ctx, acc, g.ID)
}

// checkOwnPlayers checks that the players are the account's own player or its guests.
func (s *Service) checkOwnPlayers(ctx context.Context, acc account.Account, playerIDs []int64) error {
	own, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return err
	}
	for _, id := range playerIDs {
		if !slices.ContainsFunc(own, func(p player.Player) bool { return p.ID == id }) {
			return fmt.Errorf("%w: player %d is neither you nor one of your guests", ErrInvalid, id)
		}
	}
	return nil
}

// invitedPlayers returns the players of the accounts to invite, by username. Only
// friends can be invited.
func (s *Service) invitedPlayers(ctx context.Context, acc account.Account, usernames []string) ([]int64, error) {
	var invited []int64
	for _, username := range usernames {
		username = account.NormalizeUsername(username)
		if username == "" {
			continue
		}
		other, err := s.store.GetAccountByUsername(ctx, username)
		if errors.Is(err, store.ErrNotFound) {
			return nil, UnknownUserError{Username: username}
		}
		if err != nil {
			return nil, err
		}
		if other.ID == acc.ID {
			return nil, ErrInviteSelf
		}
		friends, err := s.areFriends(ctx, acc.ID, other.ID)
		if err != nil {
			return nil, err
		}
		if !friends {
			return nil, NotFriendError{Username: username}
		}
		invited = append(invited, other.PlayerID)
	}
	return invited, nil
}

func (s *Service) Game(ctx context.Context, acc account.Account, id int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, id)
	if err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, g)
}

// ListGames returns the most recent games with a player of the account, newest first.
func (s *Service) ListGames(ctx context.Context, acc account.Account, limit int64) ([]GameView, error) {
	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, limit)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, acc, games)
}

// Invitations returns the games the account was invited to and hasn't answered yet.
func (s *Service) Invitations(ctx context.Context, acc account.Account) ([]GameView, error) {
	games, err := s.store.ListInvitedGames(ctx, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	open := games[:0]
	for _, g := range games {
		if !g.Aborted() {
			open = append(open, g)
		}
	}
	return s.views(ctx, acc, open)
}

// AcceptInvitation accepts the invitation to a game. The player throws on the board if
// one is given, one of the account's or one the others throw on; its cameras then report
// their darts.
func (s *Service) AcceptInvitation(ctx context.Context, acc account.Account, gameID int64, boardID *int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if !v.Invited {
		return GameView{}, ErrNotFound
	}
	if err := s.checkSeatBoard(ctx, acc, gameSeat(v.Game), boardID); err != nil {
		return GameView{}, err
	}
	if err := s.checkPlayersFree(ctx, acc, []int64{acc.PlayerID}, 0); err != nil {
		return GameView{}, err
	}
	if err := s.completeFinished(ctx, acc); err != nil {
		return GameView{}, err
	}
	if err := s.store.AcceptInvitation(ctx, gameID, acc.PlayerID, boardID); err != nil {
		return GameView{}, err
	}
	if err := s.takeBoard(ctx, boardID, gameID); err != nil {
		return GameView{}, err
	}
	if err := s.startGame(ctx, gameID); err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	return s.Game(ctx, acc, gameID)
}

// DeclineInvitation removes the account's player from a game it was invited to.
func (s *Service) DeclineInvitation(ctx context.Context, acc account.Account, gameID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return err
	}
	if !v.Invited {
		return ErrNotFound
	}
	if err := s.store.RemoveParticipant(ctx, gameID, acc.PlayerID); err != nil {
		return err
	}
	if err := s.startGame(ctx, gameID); err != nil {
		return err
	}
	s.notify(gameID)
	return nil
}

// WithdrawInvitation lets a player of the game remove an invited player who hasn't
// accepted yet. Returns ErrNotFound if the game was deleted because no player is left.
func (s *Service) WithdrawInvitation(ctx context.Context, acc account.Account, gameID, playerID int64) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if v.Invited || v.Game.Aborted() {
		return GameView{}, ErrForbidden
	}
	if !v.Pending(playerID) {
		return GameView{}, fmt.Errorf("%w: player %d has no pending invitation", ErrInvalid, playerID)
	}
	if err := s.store.RemoveParticipant(ctx, gameID, playerID); err != nil {
		return GameView{}, err
	}
	if err := s.startGame(ctx, gameID); err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	return s.Game(ctx, acc, gameID)
}

// startGame records when the game became ready, the start of its first turn, and sets
// the timer of that turn. Call it after every change that can complete the lobby:
// creating the game, accepting, declining or withdrawing an invitation, joining, starting
// a match. It does nothing for a game that isn't ready or started already. The caller
// holds s.mu.
func (s *Service) startGame(ctx context.Context, gameID int64) error {
	g, err := s.store.GetGame(ctx, gameID)
	if errors.Is(err, store.ErrNotFound) { // nobody is left in it
		return nil
	}
	if err != nil || g.StartedAt != nil || !g.Ready() {
		return err
	}
	now := s.now()
	g.StartedAt = &now
	if err := s.store.StartGame(ctx, &g); err != nil {
		return err
	}
	s.watchTurn(g)
	return nil
}

// abortGame marks the game as aborted and tells its subscribers. The caller holds s.mu.
func (s *Service) abortGame(ctx context.Context, g *game.Game) error {
	now := s.now()
	g.AbortedAt = &now
	if err := s.store.AbortGame(ctx, g); err != nil {
		return err
	}
	s.notifyGame(*g)
	return nil
}

// DecideBullOff settles a tie when bulling off: the players found playerID closer, so
// they start instead of throwing again. Without cameras the darts are only entered by
// field, e.g. both hit the bull.
func (s *Service) DecideBullOff(ctx context.Context, acc account.Account, gameID, playerID int64) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		if !slices.Contains(v.Common().BullOffTie, playerID) {
			return fmt.Errorf("%w: player %d isn't tied when bulling off", ErrInvalid, playerID)
		}
		g.BullOffCloser = &playerID
		return s.store.SetBullOffCloser(ctx, g)
	})
}

// AddDart registers the next dart of a game, for the player who is up, who has to be one
// of the account's (ErrForbidden otherwise: nobody enters darts for another account's
// player; the cameras of a board do, see AddBoardDart). The dart is only stored if the
// game accepts it; otherwise the error wraps dartboard.ErrInvalid or turn.ErrGameOver.
// A missing multiplier counts as single. Positions only come from the cameras
// (AddBoardDart), so a position given here is dropped.
func (s *Service) AddDart(ctx context.Context, acc account.Account, gameID int64, d throw.Dart) (GameView, error) {
	d.X, d.Y = nil, nil
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		if cur := v.Common().CurrentPlayerID; cur != nil && !v.IsMine(*cur) {
			return ErrForbidden
		}
		return s.storeDart(ctx, g, v, d)
	})
}

func (s *Service) storeDart(ctx context.Context, g *game.Game, v GameView, d throw.Dart) error {
	d.ID, d.GameID, d.CreatedAt = 0, g.ID, time.Time{} // the database sets the time
	normalizeDart(&d)
	if cur := v.Common().CurrentPlayerID; cur != nil {
		d.PlayerID = *cur // the replay below refuses the dart if the game is over
	}
	// The stored darts were accepted before, so an error can only come from the new
	// one, e.g. turn.ErrWaitingForTakeout.
	if _, err := g.Play(append(v.Darts, d)); err != nil {
		return err
	}
	return s.store.AddDart(ctx, &d)
}

// UndoDart removes the last entry of the game's log, if the last dart is one of the
// account's players' (ErrForbidden otherwise). It may be a takeout: then the player
// waits for it again.
func (s *Service) UndoDart(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		if v.Completed {
			return ErrCompleted
		}
		if !v.MayUndo() {
			if len(v.Darts) == 0 {
				return ErrNothingToUndo
			}
			return ErrForbidden
		}
		err := s.store.DeleteLastDart(ctx, g.ID)
		if errors.Is(err, store.ErrNotFound) {
			return ErrNothingToUndo
		}
		return err
	})
}

// EditDart corrects what an earlier dart hit, e.g. when the cameras or a player got it
// wrong. Only the account of the player who threw it may (ErrForbidden otherwise). The
// whole game is replayed with the correction. It is refused if the game doesn't add up
// anymore (the replay's error), or if the correction ends a turn at a different dart,
// so that later darts would count for another player (ErrChangesThrowers).
func (s *Service) EditDart(ctx context.Context, acc account.Account, gameID, dartID int64, d throw.Dart) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		if v.Completed {
			return ErrCompleted
		}
		darts := slices.Clone(v.Darts)
		i := slices.IndexFunc(darts, func(e throw.Dart) bool { return e.ID == dartID && e.IsDart() })
		if i < 0 {
			return ErrNotFound
		}
		if !v.MayEdit(i) {
			return ErrForbidden
		}
		normalizeDart(&d)
		d.ID, d.GameID, d.PlayerID = dartID, g.ID, darts[i].PlayerID
		darts[i] = d
		if _, err := g.Play(darts); errors.Is(err, turn.ErrWrongThrower) {
			return ErrChangesThrowers
		} else if err != nil {
			return err
		}
		return s.store.UpdateDart(ctx, &d)
	})
}

// normalizeDart completes a dart: a dart without a segment counts as a single (side
// unknown). Takeouts go through Takeout.
func normalizeDart(d *throw.Dart) {
	d.Takeout, d.Leave, d.Skip = false, false, false
	if d.Segment == dartboard.NoSegment {
		d.Segment = dartboard.Single
	}
}

// Takeout records that the darts were pulled out, so the next player throws. It only
// matters for games with a board (see game.Game.WaitsForTakeout) and only after darts
// were thrown in the current turn; otherwise nothing happens.
func (s *Service) Takeout(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		return s.storeTakeout(ctx, g, v)
	})
}

func (s *Service) storeTakeout(ctx context.Context, g *game.Game, v GameView) error {
	if c := v.Common(); c.GameOver || (!c.WaitingForTakeout && c.DartsInTurn == 0) {
		return nil
	}
	return s.store.AddDart(ctx, &throw.Dart{GameID: g.ID, Takeout: true})
}

// changeGame applies change to a game the account may throw in and tells the game's
// subscribers. Changes are applied one at a time, so change sees the current darts in
// v and two darts can't both pass the replay check against the same state.
func (s *Service) changeGame(ctx context.Context, acc account.Account, gameID int64, change func(g *game.Game, v GameView) error) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	g := v.Game
	if err := change(&g, v); err != nil {
		return GameView{}, err
	}
	s.notifyGame(g)
	return s.view(ctx, acc, g)
}

// changeByCameras is changeGame for what a board's cameras report: it isn't done by an
// account, so there are no rights to check and the view has nothing of a viewer.
func (s *Service) changeByCameras(ctx context.Context, gameID int64, change func(g *game.Game, v GameView) error) (GameView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.store.GetGame(ctx, gameID)
	if err != nil {
		return GameView{}, err
	}
	v, err := s.build(ctx, g)
	if err != nil {
		return GameView{}, err
	}
	if err := change(&g, v); err != nil {
		return GameView{}, err
	}
	s.notifyGame(g)
	return s.build(ctx, g)
}

// throwableGame returns the view of the game if the account may enter darts right now.
// The caller holds s.mu.
func (s *Service) throwableGame(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if v.Game.Aborted() {
		return GameView{}, ErrAborted
	}
	if !v.CanThrow {
		return GameView{}, ErrForbidden
	}
	if !v.Ready {
		return GameView{}, ErrNotReady
	}
	return v, nil
}

func (s *Service) views(ctx context.Context, acc account.Account, games []game.Game) ([]GameView, error) {
	views := []GameView{}
	for _, g := range games {
		v, err := s.view(ctx, acc, g)
		if err != nil {
			return nil, err
		}
		views = append(views, v)
	}
	return views, nil
}

// build computes everything of a game's view that doesn't depend on who looks at it.
func (s *Service) build(ctx context.Context, g game.Game) (GameView, error) {
	v := GameView{Game: g}
	v.Ready = g.Ready()

	var err error
	if v.Darts, err = s.store.ListDarts(ctx, g.ID); err != nil {
		return GameView{}, err
	}
	if v.State, err = g.Play(v.Darts); err != nil {
		return GameView{}, err
	}
	v.Completed = g.Completed(v.Darts, v.Common(), s.now())
	if d, ok := g.TurnDeadline(v.Darts, v.Common()); ok {
		if earliest := s.started.Add(time.Duration(g.TurnSeconds) * time.Second); d.Before(earliest) {
			d = earliest
		}
		v.TurnDeadline = &d
	}
	if v.Players, err = s.store.GetPlayers(ctx, g.PlayerIDs()); err != nil {
		return GameView{}, err
	}
	for _, p := range g.Participants {
		if p.BoardID == nil {
			continue
		}
		if v.BoardNames == nil {
			v.BoardNames = map[int64]string{}
		}
		if b, err := s.store.GetBoard(ctx, *p.BoardID); err == nil {
			v.BoardNames[p.PlayerID] = b.Name
		} else if !errors.Is(err, store.ErrNotFound) {
			return GameView{}, err
		}
	}
	return v, nil
}

// participation is how an account takes part in a game or tournament: through the
// players it controls (account.Account.Controls).
type participation struct {
	mine    []int64 // the account's players that take part
	invited bool    // its own player hasn't answered the invitation yet
	canPlay bool    // one of its players accepted: it may throw, or start matches
}

// participationOf works out how the account takes part, from the players of a game or
// tournament and whose invitation is pending; ErrNotFound if it takes no part, so the
// game's existence isn't revealed. Nothing can be done in one that was aborted.
func participationOf(acc account.Account, players []player.Player, pending func(playerID int64) bool, aborted bool) (participation, error) {
	var pt participation
	accepted := false
	for _, p := range players {
		if acc.Controls(p) {
			pt.mine = append(pt.mine, p.ID)
			accepted = accepted || !pending(p.ID)
		}
	}
	if len(pt.mine) == 0 {
		return participation{}, ErrNotFound
	}
	pt.invited = pending(acc.PlayerID) && !aborted
	pt.canPlay = accepted && !aborted
	return pt, nil
}

// view builds the view of a game for an account, ErrNotFound if the account takes no part
// in it: no player of the account (its own or a guest) is a participant.
func (s *Service) view(ctx context.Context, acc account.Account, g game.Game) (GameView, error) {
	v, err := s.build(ctx, g)
	if err != nil {
		return GameView{}, err
	}
	pt, err := participationOf(acc, v.Players, v.Pending, g.Aborted())
	if err != nil {
		return GameView{}, err
	}
	v.Mine, v.Invited, v.CanThrow = pt.mine, pt.invited, pt.canPlay
	if slices.Contains(v.Mine, acc.PlayerID) {
		v.OwnPlayerID = acc.PlayerID
	}
	if !slices.ContainsFunc(v.Mine, func(id int64) bool { return !v.Common().Records[id].Left }) {
		v.HasLeft, v.CanThrow = true, false
	}
	if v.Common().GameOver && v.OwnPlayerID != 0 {
		ratings, err := s.store.ListRatingsBy(ctx, g.ID, v.OwnPlayerID)
		if err != nil {
			return GameView{}, err
		}
		for _, r := range ratings {
			if v.Ratings == nil {
				v.Ratings = map[int64]bool{}
			}
			v.Ratings[r.RatedID] = r.Up
		}
	}
	if v.Invited || (v.CanThrow && !v.Common().GameOver) {
		if v.Boards, err = s.boardChoices(ctx, acc, g); err != nil {
			return GameView{}, err
		}
	}
	return v, nil
}
