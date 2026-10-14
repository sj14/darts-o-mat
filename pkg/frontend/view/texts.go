package view

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/auth"
	"github.com/sj14/darts-o-mat/pkg/i18n"
)

// Texts are the texts of the web interface, one method per text (see pkg/i18n). English
// (textsEN) implements all of them; the other languages embed it and override what is
// translated, the rest stays English. TestTranslations lists what isn't translated yet.
//
// Not all pages use it yet: so far the frame of the pages and the game page (2026-10-14).
type Texts interface {
	// Numbers.
	Decimal(f float64) string // one decimal place
	Percent(ratio float64) string

	// Frame of the pages.
	NavStats() string
	NavFriends() string
	NavBoards() string
	Logout() string
	Language() string
	LanguageBrowser() string
	SoundOn() string
	SoundOff() string
	SoundBlocked() string

	// Game modes and settings.
	AroundTheClock() string
	WithBull() string
	MultiplierSkips() string
	FirstToSets(sets, legs int) string

	// Game page: title and lobby.
	ToTournament() string
	BullOff() string
	SetN(n int) string
	LegN(n int) string
	RoundN(n int) string
	InvitedToGame() string
	Accept() string
	Decline() string
	GameStartsWhenAccepted() string
	GuestMark() string
	Joined() string
	Accepted() string
	Invited() string
	Uninvite() string
	AbortGame() string
	AbortGameConfirm() string

	// Game page: playing.
	Wins(name string) string
	NewGame() string
	GameAborted() string
	CheckoutSuggestion() string
	Checkout() string
	PullDarts() string
	ThenSet(n int) string
	ThenLeg(n int) string
	ThenPlayer(name string) string
	CameraSeesTakeout() string
	DartsPulled() string
	UndoLastDart() string
	Starts(name string) string
	SetTo(name string) string
	LegTo(name string) string
	Bust() string
	Busted() string
	Win() string
	ToThrow() string
	Next() string
	SetsWon(n int) string
	LegsWon(n int) string
	Remaining() string
	Average3() string
	AverageHint() string
	Target() string
	HitRate() string

	// Game page: entering darts.
	Multiplier() string
	Single() string
	Double() string
	Triple() string
	Bull() string
	Bullseye() string
	Miss() string
	Hit() string
	OtherNumber() string
	Correct() string
	CorrectDart() string
	DartBy(dart, name string) string
	CorrectHint() string
	Cancel() string
	BullOffRule(name string) string
	BullOffAgain() string
	BullOffTie() string
	WasCloser(name string) string

	// Game page: history and statistics.
	History() string
	Statistics() string
	Player() string
	Round() string
	Darts() string
	Points() string
	Highest() string
	Hits() string
	Finished() string

	// Game page: errors.
	ErrInput() string
	ErrTripleBull() string
	ErrNoSuchField() string
	ErrChangesThrowers() string
	ErrEndsEarlier() string
	ErrNoBullOffTie() string
	ErrNoInvitation() string
	ErrGameOver() string
	ErrPullDartsFirst() string
	ErrNotReady() string
	ErrAborted() string
	ErrNothingToUndo() string
	ErrForbidden() string
}

// T returns the texts in the language of the request (see i18n.From).
func T(ctx context.Context) Texts {
	return textsOf(i18n.From(ctx))
}

func textsOf(l i18n.Lang) Texts {
	if l == i18n.German {
		return textsDE{}
	}
	return textsEN{}
}

// languageName is a language in its own language, as the language choice shows it.
func languageName(l i18n.Lang) string {
	switch l {
	case i18n.German:
		return "Deutsch"
	}
	return "English"
}

// accountLanguage is the language the logged-in account chose, empty for the browser's.
func accountLanguage(ctx context.Context) i18n.Lang {
	if l, ok := i18n.Parse(auth.Account(ctx).Language); ok {
		return l
	}
	return ""
}
