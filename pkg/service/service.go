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
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/notify"
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
	State       game.State        `json:"state"` // x01.State or aroundtheclock.State, depending on the mode
	// Ready is true once no invitation is pending anymore; only then darts can be thrown.
	Ready bool `json:"ready"`

	// The following fields depend on the account the view was created for.
	IsOwner  bool `json:"is_owner"`
	CanThrow bool `json:"can_throw"` // may enter and undo darts (once the game is ready, never after it was aborted)
	Invited  bool `json:"invited"`   // has a pending invitation to answer
	// Boards are the owner's boards a running game without a board can move to (UseBoard).
	Boards []BoardChoice `json:"boards,omitempty"`
}

// Common returns the part of the game's state that is the same for all modes.
func (v GameView) Common() turn.State {
	if v.State == nil {
		return turn.State{}
	}
	return v.State.Common()
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
	if g.Sets == 0 {
		g.Sets = 1 // no sets unless asked otherwise
	}

	if err := s.checkOwnPlayers(ctx, acc, g.PlayerIDs); err != nil {
		return GameView{}, err
	}
	invited, err := s.invitedPlayers(ctx, acc, inviteUsernames)
	if err != nil {
		return GameView{}, err
	}
	g.PlayerIDs = append(g.PlayerIDs, invited...)
	if err := s.checkBoard(ctx, acc, g.BoardID); err != nil {
		return GameView{}, err
	}

	if err := g.Validate(); err != nil {
		return GameView{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := s.store.CreateGame(ctx, &g, invited, game.InvitationPending); err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, g)
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

// checkBoard checks that the board, if any, is one of the account's.
func (s *Service) checkBoard(ctx context.Context, acc account.Account, boardID *int64) error {
	if boardID == nil {
		return nil
	}
	b, err := s.store.GetBoard(ctx, *boardID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && b.AccountID != acc.ID) {
		return fmt.Errorf("%w: unknown board", ErrInvalid)
	}
	return err
}

func (s *Service) Game(ctx context.Context, acc account.Account, id int64) (GameView, error) {
	g, err := s.store.GetGame(ctx, id)
	if err != nil {
		return GameView{}, err
	}
	return s.view(ctx, acc, g)
}

// ListGames returns the most recent games the account created or was invited to, newest first.
func (s *Service) ListGames(ctx context.Context, acc account.Account, limit int64) ([]GameView, error) {
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
	if v.Common().GameOver {
		return GameView{}, turn.ErrGameOver
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	v.Game.AbortedAt = &now
	if err := s.store.AbortGame(ctx, &v.Game); err != nil {
		return GameView{}, err
	}
	s.notifyGame(v.Game)
	return s.view(ctx, acc, v.Game)
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

// AddDart registers the next dart of a game. The dart is only stored if the
// game accepts it; otherwise the error wraps throw.ErrInvalid or turn.ErrGameOver.
// A missing multiplier counts as single. Positions only come from the cameras
// (AddBoardDart), so a position given here is dropped.
func (s *Service) AddDart(ctx context.Context, acc account.Account, gameID int64, d throw.Dart) (GameView, error) {
	d.X, d.Y = nil, nil
	return s.addDart(ctx, acc, gameID, d)
}

func (s *Service) addDart(ctx context.Context, acc account.Account, gameID int64, d throw.Dart) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		d.ID, d.GameID, d.CreatedAt = 0, g.ID, time.Time{} // the database sets the time
		normalizeDart(&d)
		// The stored darts were accepted before, so an error can only come from the new
		// one, e.g. turn.ErrWaitingForTakeout.
		if _, err := g.Play(append(v.Darts, d)); err != nil {
			return err
		}
		return s.store.AddDart(ctx, &d)
	})
}

// UndoDart removes the last entry of the game's log. It may be a takeout: then the
// player waits for it again.
func (s *Service) UndoDart(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, _ GameView) error {
		err := s.store.DeleteLastDart(ctx, g.ID)
		if errors.Is(err, store.ErrNotFound) {
			return ErrNothingToUndo
		}
		return err
	})
}

// EditDart corrects what an earlier dart hit, e.g. when the cameras or a player got it
// wrong. The whole game is replayed with the correction. It is refused if the game
// doesn't add up anymore (the replay's error), or if the correction ends a turn at a
// different dart, so that later darts would count for another player (ErrChangesThrowers).
func (s *Service) EditDart(ctx context.Context, acc account.Account, gameID, dartID int64, d throw.Dart) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		darts := slices.Clone(v.Darts)
		i := slices.IndexFunc(darts, func(e throw.Dart) bool { return e.ID == dartID && !e.Takeout })
		if i < 0 {
			return ErrNotFound
		}
		normalizeDart(&d)
		d.ID, d.GameID = dartID, g.ID
		darts[i] = d
		after, err := g.Play(darts)
		if err != nil {
			return err
		}
		if !slices.Equal(v.Common().Throwers, after.Common().Throwers) {
			return ErrChangesThrowers
		}
		return s.store.UpdateDart(ctx, &d)
	})
}

// normalizeDart completes a dart: a dart without a segment counts as a single (side
// unknown). Takeouts go through Takeout.
func normalizeDart(d *throw.Dart) {
	d.Takeout = false
	if d.Segment == throw.NoSegment {
		d.Segment = throw.Single
	}
}

// Takeout records that the darts were pulled out, so the next player throws. It only
// matters for games on a board (see game.Game.WaitsForTakeout) and only after darts
// were thrown in the current turn; otherwise nothing happens.
func (s *Service) Takeout(ctx context.Context, acc account.Account, gameID int64) (GameView, error) {
	return s.changeGame(ctx, acc, gameID, func(g *game.Game, v GameView) error {
		if c := v.Common(); c.GameOver || (!c.WaitingForTakeout && c.DartsInTurn == 0) {
			return nil
		}
		return s.store.AddDart(ctx, &throw.Dart{GameID: g.ID, Takeout: true})
	})
}

// changeGame applies change to a game the account may throw in and tells the game's
// subscribers. Changes are applied one at a time, so change sees the current darts in
// v and two darts can't both pass the replay check against the same state.
func (s *Service) changeGame(ctx context.Context, acc account.Account, gameID int64, change func(g *game.Game, v GameView) error) (GameView, error) {
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
	if err := change(&g, v); err != nil {
		return GameView{}, err
	}
	s.notifyGame(g)
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

	v := GameView{Game: g, Invitations: invitations, IsOwner: g.OwnerAccountID == acc.ID}
	v.Ready = !slices.ContainsFunc(invitations, func(inv game.Invitation) bool { return inv.Status == game.InvitationPending })
	var mine *game.InvitationStatus
	if inv := v.InvitationOf(acc.PlayerID); inv != nil {
		mine = &inv.Status
	}
	if v.CanThrow, v.Invited, err = access(v.IsOwner, mine, g.Aborted()); err != nil {
		return GameView{}, err
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

	v.Players, err = s.store.GetPlayers(ctx, g.PlayerIDs)
	if err != nil {
		return GameView{}, err
	}
	if v.IsOwner && g.BoardID == nil && !g.Aborted() && !v.Common().GameOver {
		if v.Boards, err = s.boardChoices(ctx, v); err != nil {
			return GameView{}, err
		}
	}
	return v, nil
}

// access returns what an account may do in a game or tournament, from its part in it:
// the owner and the players who accepted their invitation (mine) may play, those
// with a pending one have to answer it first; it is hidden from everyone else
// (ErrNotFound). Once it was aborted, nobody plays or answers anymore.
func access(owner bool, mine *game.InvitationStatus, aborted bool) (canPlay, invited bool, err error) {
	switch {
	case owner:
		canPlay = true
	case mine == nil:
		return false, false, ErrNotFound
	case *mine == game.InvitationAccepted:
		canPlay = true
	default:
		invited = true
	}
	if aborted {
		return false, false, nil
	}
	return canPlay, invited, nil
}
