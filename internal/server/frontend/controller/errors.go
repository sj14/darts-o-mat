package controller

import (
	"errors"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

// errorText returns the message of an error that has one of its own, the same on every
// page, like the API maps errors to status codes in one place. It reports false for the
// others: a plain service.ErrInvalid, whose message depends on the form it came from, and
// internal errors.
func errorText(t view.Texts, err error) (string, bool) {
	if unknown, ok := errors.AsType[service.UnknownUserError](err); ok {
		return t.ErrUnknownUser(unknown.Username), true
	}
	if notFriend, ok := errors.AsType[service.NotFriendError](err); ok {
		return t.ErrNotFriend(notFriend.Username), true
	}
	if busy, ok := errors.AsType[service.PlayerBusyError](err); ok {
		return t.ErrPlayerBusy(busy.Player), true
	}
	if errors.Is(err, page.ErrTooLong) {
		return t.ErrPageTooLong(page.MaxLen), true
	}
	if errors.Is(err, tournament.ErrPlayerCount) {
		return t.ErrTournamentPlayers(tournament.MinPlayers, tournament.MaxPlayers), true
	}
	texts := []struct {
		err  error
		text func() string
	}{
		// Accounts and friends.
		{service.ErrUsernameTaken, t.ErrUsernameTaken},
		{service.ErrEmailTaken, t.ErrEmailTaken},
		{account.ErrInvalidEmail, t.ErrInvalidEmail},
		{service.ErrCaptcha, t.ErrCaptcha},
		{service.ErrRegistrationClosed, t.RegistrationClosed},
		{service.ErrTooManyLogins, t.ErrTooManyLogins},
		{service.ErrWrongPassword, t.ErrWrongPassword},
		{service.ErrAlreadyFriends, t.ErrAlreadyFriends},
		{service.ErrInviteSelf, t.ErrInviteSelf},
		{service.ErrGuestNameTaken, t.ErrGuestNameTaken},
		{service.ErrGuestBusy, t.ErrGuestBusy},
		// Boards and lobbies.
		{service.ErrBoardBusy, t.ErrBoardBusy},
		{service.ErrBoardRequired, t.ErrBoardRequired},
		{service.ErrNoSeat, t.ErrNoSeat},
		// Games.
		{service.ErrChangesThrowers, t.ErrChangesThrowers},
		{turn.ErrGameOver, t.ErrGameOver},
		{turn.ErrWaitingForTakeout, t.ErrPullDartsFirst},
		{service.ErrNotReady, t.ErrNotReady},
		{service.ErrAborted, t.ErrAborted},
		{service.ErrCompleted, t.ErrCompleted},
		{service.ErrNotOver, t.ErrNotOver},
		{service.ErrNothingToUndo, t.ErrNothingToUndo},
		{service.ErrCantLeave, t.ErrCantLeave},
		{service.ErrForbidden, t.ErrForbidden},
	}
	for _, e := range texts {
		if errors.Is(err, e.err) {
			return e.text(), true
		}
	}
	return "", false
}

// alertOf is the alert for err with the message msg: a service.PlayerBusyError links the
// game or tournament the player is in, if the account may look into it.
func alertOf(err error, msg string) view.Alert {
	busy, _ := errors.AsType[service.PlayerBusyError](err)
	return view.Alert{Text: msg, Game: busy.GameID, Tournament: busy.TournamentID}
}
