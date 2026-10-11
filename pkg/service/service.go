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

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
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
	// ErrChangesThrowers means a correction would change who threw later darts,
	// e.g. a bust that ends a turn early: the later darts have to be taken back instead.
	ErrChangesThrowers = fmt.Errorf("%w: the correction would change who threw the later darts", ErrInvalid)
	ErrInviteSelf      = fmt.Errorf("%w: you can't invite yourself", ErrInvalid)
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
}

type Service struct {
	store *store.Store
	opts  Options
	// mu serializes changes to the darts and players of games, so two darts
	// can't both pass the replay check against the same previous state.
	mu sync.Mutex
	// hub notifies about changed games.
	hub hub
}

func New(store *store.Store, opts Options) *Service {
	return &Service{store: store, opts: opts}
}

func (s *Service) RegistrationOpen() bool {
	return s.opts.Registration
}

// GameView is a game together with everything needed to show it to an account.
type GameView struct {
	Game        game.Game         `json:"game"`
	Board       *board.Board      `json:"board,omitempty"` // the board it's played on, if any
	Players     []player.Player   `json:"players"`         // in throwing order
	Invitations []game.Invitation `json:"invitations"`
	Darts       []throw.Dart      `json:"darts"` // the log: darts and takeouts
	State       any               `json:"state"` // x01.State or aroundtheclock.State, depending on the mode
	// Ready is true once no invitation is pending anymore; only then darts can be thrown.
	Ready bool `json:"ready"`

	// The following fields depend on the account the view was created for.
	IsOwner  bool `json:"is_owner"`
	CanThrow bool `json:"can_throw"` // may enter and undo darts (once the game is ready, never after it was aborted)
	Invited  bool `json:"invited"`   // has a pending invitation to answer
}

// InvitationOf returns the invitation of a player, nil if the player wasn't invited (owner or guest).
func (v GameView) InvitationOf(playerID int64) *game.Invitation {
	for _, inv := range v.Invitations {
		if inv.PlayerID == playerID {
			return &inv
		}
	}
	return nil
}

// CreateGame creates a game owned by the account. PlayerIDs may only contain the
// account's own player and its guests; friends are invited by username
// and added to the end of the throwing order.
func (s *Service) CreateGame(ctx context.Context, acc account.Account, g game.Game, inviteUsernames []string) (GameView, error) {
	g.ID = 0
	g.OwnerAccountID = acc.ID
	if g.Legs == 0 {
		g.Legs = 1 // a single leg unless asked otherwise
	}

	own, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return GameView{}, err
	}
	for _, id := range g.PlayerIDs {
		if !containsPlayer(own, id) {
			return GameView{}, fmt.Errorf("%w: player %d is neither you nor one of your guests", ErrInvalid, id)
		}
	}

	var invited []int64
	for _, username := range inviteUsernames {
		username = account.NormalizeUsername(username)
		if username == "" {
			continue
		}
		other, err := s.store.GetAccountByUsername(ctx, username)
		if errors.Is(err, store.ErrNotFound) {
			return GameView{}, UnknownUserError{Username: username}
		}
		if err != nil {
			return GameView{}, err
		}
		if other.ID == acc.ID {
			return GameView{}, ErrInviteSelf
		}
		friends, err := s.areFriends(ctx, acc.ID, other.ID)
		if err != nil {
			return GameView{}, err
		}
		if !friends {
			return GameView{}, NotFriendError{Username: username}
		}
		g.PlayerIDs = append(g.PlayerIDs, other.PlayerID)
		invited = append(invited, other.PlayerID)
	}

	if g.BoardID != nil {
		b, err := s.store.GetBoard(ctx, *g.BoardID)
		if errors.Is(err, store.ErrNotFound) || (err == nil && b.AccountID != acc.ID) {
			return GameView{}, fmt.Errorf("%w: unknown board", ErrInvalid)
		}
		if err != nil {
			return GameView{}, err
		}
	}

	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.CreateGame(ctx, &g, invited); err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, g)
}

func (s *Service) Game(ctx context.Context, acc account.Account, id int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, id)
	if err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, g)
}

// ListGames returns the most recent games the account created or was invited to, newest first.
func (s *Service) ListGames(ctx context.Context, acc account.Account, limit int) ([]GameView, error) {
	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, limit)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, acc, games)
}

// Invitations returns the games the account was invited to and hasn't answered yet.
func (s *Service) Invitations(ctx context.Context, acc account.Account) ([]GameView, error) {
	invitations, err := s.store.ListPendingInvitations(ctx, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	var games []game.Game
	for _, inv := range invitations {
		g, err := s.store.GetGame(ctx, inv.GameID)
		if err != nil {
			return nil, err
		}
		if !g.Aborted() {
			games = append(games, g)
		}
	}
	return s.views(ctx, acc, games)
}

func (s *Service) AcceptInvitation(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	if err := s.store.AcceptInvitation(ctx, gameID, acc.PlayerID); err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	return s.Game(ctx, acc, gameID)
}

// DeclineInvitation removes the account's player from a game it was invited to.
func (s *Service) DeclineInvitation(ctx context.Context, acc account.Account, gameID int64) error {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return err
	}
	if !v.Invited {
		return ErrNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.store.RemoveInvitedPlayer(ctx, gameID, acc.PlayerID); err != nil {
		return err
	}
	s.notify(gameID)
	return nil
}

// WithdrawInvitation lets the owner remove an invited player who hasn't accepted yet.
// Returns ErrNotFound if the game was deleted because no player is left.
func (s *Service) WithdrawInvitation(ctx context.Context, acc account.Account, gameID, playerID int64) (GameView, error) {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if !v.IsOwner {
		return GameView{}, ErrForbidden
	}
	inv := v.InvitationOf(playerID)
	if inv == nil || inv.Status != game.InvitationPending {
		return GameView{}, fmt.Errorf("%w: player %d has no pending invitation", ErrInvalid, playerID)
	}

	s.mu.Lock()
	err = s.store.RemoveInvitedPlayer(ctx, gameID, playerID)
	s.mu.Unlock()
	if err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	return s.Game(ctx, acc, gameID)
}

// AbortGame ends a running game without a winner. Only the owner may abort it; the
// game stays with its darts, but takes no more and doesn't count in the statistics.
func (s *Service) AbortGame(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	if !v.IsOwner {
		return GameView{}, ErrForbidden
	}
	if v.Game.Aborted() {
		return GameView{}, ErrAborted
	}
	if _, over := v.turnProgress(); over {
		return GameView{}, turn.ErrGameOver
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	v.Game.AbortedAt = &now
	if err := s.store.AbortGame(ctx, &v.Game); err != nil {
		return GameView{}, err
	}
	s.notify(gameID)
	return s.view(ctx, acc, v.Game)
}

// DecideBullOff settles a tie when bulling off: the players found playerID closer, so
// they start instead of throwing again. Without cameras the darts are only entered by
// field, e.g. both hit the bull.
func (s *Service) DecideBullOff(ctx context.Context, acc account.Account, gameID, playerID int64) (GameView, error) {
	g, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.view(ctx, acc, g)
	if err != nil {
		return GameView{}, err
	}
	if !slices.Contains(bullOffTie(v.State), playerID) {
		return GameView{}, fmt.Errorf("%w: player %d isn't tied when bulling off", ErrInvalid, playerID)
	}
	g.BullOffCloser = &playerID
	if err := s.store.SetBullOffCloser(ctx, &g); err != nil {
		return GameView{}, err
	}
	s.notify(g.ID)
	return s.view(ctx, acc, g)
}

// AddDart registers the next dart of a game. The dart is only stored if the
// game accepts it; otherwise the error wraps throw.ErrInvalid or turn.ErrGameOver.
// A missing multiplier counts as single. Positions only come from the cameras
// (AddBoardDart), so a position given here is dropped.
func (s *Service) AddDart(ctx context.Context, acc account.Account, gameID int64, d throw.Dart) (GameView, error) {
	d.X, d.Y = nil, nil
	return s.addDart(ctx, acc, gameID, d)
}

func (s *Service) addDart(ctx context.Context, acc account.Account, gameID int64, d throw.Dart) (GameView, error) {
	g, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}

	d.ID = 0
	d.GameID = g.ID
	normalizeDart(&d)

	s.mu.Lock()
	defer s.mu.Unlock()

	darts, err := s.store.ListDarts(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}
	// The stored darts were accepted before, so an error can only come from the new one,
	// e.g. turn.ErrWaitingForTakeout.
	if _, err := g.Play(append(darts, d)); err != nil {
		return GameView{}, err
	}

	if err := s.store.AddDart(ctx, &d); err != nil {
		return GameView{}, err
	}
	s.notify(g.ID)
	return s.view(ctx, acc, g)
}

func (s *Service) UndoDart(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	g, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// The last entry may be a takeout: then the player waits for it again.
	err = s.store.DeleteLastDart(ctx, g.ID)
	if errors.Is(err, store.ErrNotFound) {
		return GameView{}, ErrNothingToUndo
	}
	if err != nil {
		return GameView{}, err
	}
	s.notify(g.ID)
	return s.view(ctx, acc, g)
}

// EditDart corrects what an earlier dart hit, e.g. when the cameras or a player got it
// wrong. The whole game is replayed with the correction. It is refused if the game
// doesn't add up anymore (the replay's error), or if the correction ends a turn at a
// different dart, so that later darts would count for another player (ErrChangesThrowers).
func (s *Service) EditDart(ctx context.Context, acc account.Account, gameID, dartID int64, d throw.Dart) (GameView, error) {
	g, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}
	normalizeDart(&d)

	s.mu.Lock()
	defer s.mu.Unlock()

	darts, err := s.store.ListDarts(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}
	i := slices.IndexFunc(darts, func(e throw.Dart) bool { return e.ID == dartID && !e.Takeout })
	if i < 0 {
		return GameView{}, ErrNotFound
	}
	before, err := g.Play(darts)
	if err != nil {
		return GameView{}, err
	}
	d.ID, d.GameID = dartID, g.ID
	darts[i] = d
	after, err := g.Play(darts)
	if err != nil {
		return GameView{}, err
	}
	if !slices.Equal(game.Throwers(before), game.Throwers(after)) {
		return GameView{}, ErrChangesThrowers
	}
	if err := s.store.UpdateDart(ctx, &d); err != nil {
		return GameView{}, err
	}
	s.notify(g.ID)
	return s.view(ctx, acc, g)
}

// normalizeDart clears what doesn't matter for a dart: a miss has no field, and a
// missing multiplier counts as single. Takeouts go through Takeout.
func normalizeDart(d *throw.Dart) {
	d.Takeout = false
	if d.Miss {
		d.Number, d.Multiplier = 0, 0
	} else if d.Multiplier == 0 {
		d.Multiplier = 1
	}
}

// Takeout records that the darts were pulled out, so the next player throws. It only
// matters for games on a board (see game.Game.WaitsForTakeout) and only after darts
// were thrown in the current turn; otherwise nothing happens.
func (s *Service) Takeout(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	g, err := s.throwableGame(ctx, acc, gameID)
	if err != nil {
		return GameView{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	v, err := s.view(ctx, acc, g)
	if err != nil {
		return GameView{}, err
	}
	thrown, over := v.turnProgress()
	if over || (!waiting(v.State) && thrown == 0) || len(v.Darts) == 0 {
		return v, nil
	}
	if err := s.store.AddDart(ctx, &throw.Dart{GameID: g.ID, Takeout: true}); err != nil {
		return GameView{}, err
	}
	s.notify(g.ID)
	return s.view(ctx, acc, g)
}

// throwableGame returns the game if the account may enter darts right now.
// A game only gets ready once, invitations can't be added later.
func (s *Service) throwableGame(ctx context.Context, acc account.Account, gameID int64) (game.Game, error) {
	v, err := s.Game(ctx, acc, gameID)
	if err != nil {
		return game.Game{}, err
	}
	if v.Game.Aborted() {
		return game.Game{}, ErrAborted
	}
	if !v.CanThrow {
		return game.Game{}, ErrForbidden
	}
	if !v.Ready {
		return game.Game{}, ErrNotReady
	}
	return v.Game, nil
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

// view builds the view of a game for an account, ErrNotFound if the account takes no part in it.
func (s *Service) view(ctx context.Context, acc account.Account, g game.Game) (GameView, error) {
	invitations, err := s.store.ListInvitations(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}

	v := GameView{Game: g, Invitations: invitations, Ready: true, IsOwner: g.OwnerAccountID == acc.ID}
	for _, inv := range invitations {
		if inv.Status == game.InvitationPending {
			v.Ready = false
		}
	}
	mine := v.InvitationOf(acc.PlayerID)
	switch {
	case v.IsOwner:
		v.CanThrow = true
	case mine == nil:
		return GameView{}, ErrNotFound
	case mine.Status == game.InvitationAccepted:
		v.CanThrow = true
	default:
		v.Invited = true
	}
	if g.Aborted() {
		v.CanThrow, v.Invited = false, false
	}

	if g.BoardID != nil {
		b, err := s.store.GetBoard(ctx, *g.BoardID)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return GameView{}, err
		}
		if err == nil {
			v.Board = &b
		}
	}

	v.Darts, err = s.store.ListDarts(ctx, g.ID)
	if err != nil {
		return GameView{}, err
	}
	v.State, err = g.Play(v.Darts)
	if err != nil {
		return GameView{}, err
	}

	players, err := s.store.GetPlayers(ctx, g.PlayerIDs)
	if err != nil {
		return GameView{}, err
	}
	v.Players = []player.Player{}
	for _, id := range g.PlayerIDs {
		for _, p := range players {
			if p.ID == id {
				v.Players = append(v.Players, p)
			}
		}
	}
	return v, nil
}

func containsPlayer(players []player.Player, id int64) bool {
	for _, p := range players {
		if p.ID == id {
			return true
		}
	}
	return false
}

// waiting reports whether the game's state waits for the darts to be pulled out.
func waiting(state any) bool {
	switch st := state.(type) {
	case x01.State:
		return st.WaitingForTakeout
	case aroundtheclock.State:
		return st.WaitingForTakeout
	}
	return false
}

// bullOffTie returns the players tied when bulling off.
func bullOffTie(state any) []int64 {
	switch st := state.(type) {
	case x01.State:
		return st.BullOffTie
	case aroundtheclock.State:
		return st.BullOffTie
	}
	return nil
}
