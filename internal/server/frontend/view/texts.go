package view

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/common/i18n"
)

// Texts are the texts of the web interface, one method per text (see internal/common/i18n). English
// (textsEN) implements all of them; the other languages embed it and override what is
// translated, the rest stays English. TestTranslations lists what isn't translated yet.
type Texts interface {
	// Numbers.
	Decimal(f float64) string // one decimal place
	Percent(ratio float64) string

	// Frame of the pages.
	NavStats() string
	NavFriends() string
	NavBoards() string
	NavSettings() string
	AccentColor() string
	AccentColorHint() string
	ColorAuto() string
	ColorName(color string) string
	Logout() string
	Language() string
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
	PlayOnBoard() string
	RemoveBoard() string
	RemoveBoardConfirm() string
	OpenGames() string
	NoOpenGames() string
	PlayersOfSeats(players, seats int) string
	JoinLobby() string
	LobbiesHint() string
	LobbiesShown(shown, total int) string
	NoMatchingGames() string
	OpenForMinutes(n int) string
	GameMode() string
	FilterAny() string
	FilterYes() string
	FilterNo() string
	FilterReset() string
	TurnLimited() string
	Strangers() string
	AllowStrangers() string
	StrangerSeats() string
	StrangersHint() string
	WaitingForSeats(n int) string
	ErrNoSeat() string
	LandingLead() string
	LandingFeatureTitle(i int) string
	LandingFeatureText(i int) string
	BoardInUse() string
	BoardInUseConfirm() string

	// Game page: playing.
	WinsGame(name string) string
	// Duration is how long a leg, set, game or tournament took, e.g. "Duration 34:12".
	Duration(clock string) string
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
	EnterDart() string
	DeletedPlayer() string
	DeleteGuest() string
	DeleteGuestConfirm(name string) string
	ErrGuestBusy() string
	DeleteAccountTitle() string
	DeleteAccountHint() string
	DeleteAccountButton() string
	DeleteAccountConfirm() string
	ErrWrongPassword() string
	LeaveGame() string
	LeaveGameConfirm() string
	PlayerLeft() string
	YouLeft() string
	ErrCantLeave() string
	HideKeypad() string
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
	ErrCompleted() string
	ErrNotOver() string
	CompleteGame() string
	NewVersion(version string) string
	CompleteHint() string
	ResultConfirmed() string
	AwaitingConfirmation() string
	ErrPullDartsFirst() string
	ErrNotReady() string
	ErrAborted() string
	ErrNothingToUndo() string
	ErrForbidden() string

	// Start page and new game.
	HomeTitle() string
	FriendRequests(n int) string
	Show() string
	Tournaments() string
	Invitations() string
	TournamentInvitations() string
	WithPlayers(names string) string
	PlayerCount(n int) string
	GuestsHint() string
	Name() string
	GuestName() string
	Add() string
	RecentGames() string
	StatusAborted() string
	StatusWaiting() string
	StatusOver() string
	StatusRunning() string
	GameFormat() string
	SingleGame() string
	Knockout() string
	RoundRobin() string
	TournamentHint(min, max int) string
	Legs() string
	Sets() string
	OneLeg() string
	LegsHint() string
	NoSets() string
	SetsHint() string
	WhoStarts() string
	BullOffOptionHint() string
	TurnLimit() string
	NoTurnLimit() string
	TurnTime(seconds int) string
	TurnLimitHint(skips int) string
	TurnTimeLeft() string
	Players() string
	Me(name string) string
	InviteFriends() string
	NoFriendsBefore() string
	NoFriendsAfter() string
	Board() string
	NoBoard() string
	X01Description() string
	AroundTheClockDescription() string
	StartScore() string
	CheckOutWith() string
	AnyDart() string
	DoubleOrTriple() string
	StartMode(mode string) string
	Rules() string
	BullAtTheEnd() string
	SkipsRule() string
	ChooseMode() string

	// Boards.
	BoardsHint() string
	CameraTokenFor(board string) string
	TokenShownOnce() string
	Server() string
	CameraToken() string
	CalibrationHintBefore() string
	CalibrationHintMiddle() string
	CameraTokenActive() string
	CreateCameraToken() string
	NewCameraToken() string
	Delete() string
	BoardNameExample() string
	BoardName() string

	// Login and registration.
	Login() string
	Register() string
	Username() string
	Password() string
	NoAccountYet() string
	UsernameHint() string
	Email() string
	EmailOptional() string
	EmailHint() string
	SaveEmail() string
	DisplayName() string
	DisplayNameHint() string
	PasswordHint() string
	CreateAccount() string
	RegistrationClosed() string
	AlreadyRegistered() string

	// Friends.
	FriendsHint() string
	SendRequest() string
	RequestsToYou() string
	AcceptRequest() string
	DeclineRequest() string
	YourFriends() string
	NoFriendsYet() string
	Remove() string
	SentRequests() string
	WithdrawRequest() string

	// Statistics.
	StatsHint() string
	NoFinishedGames() string
	Heatmap() string
	AllModes() string
	BoardRequired() string
	BoardOnlyOption() string
	BoardOnlyHint() string
	NeedsBoard() string
	ErrBoardRequired() string
	HeatmapDarts(n int) string
	HeatmapNone() string
	Count180() string
	GamesWinsLegs() string
	SetsWonLabel() string
	TournamentsWon() string
	HighestFinish() string
	BestLeg() string
	AvgLegTime() string
	AvgGameTime() string
	NDarts(n int) string
	Games() string
	DartsHits() string

	// Tournaments.
	InvitedToTournament() string
	WaitingFor(names string) string
	WinsTournament(name string) string
	LeaveTournament() string
	LeaveTournamentConfirm() string
	Bracket() string
	RoundName(rounds, round int) string
	Bye() string
	Open() string
	StartGame() string
	RunningToGame() string
	ToGame() string
	Table() string
	Wins() string
	Difference() string
	TieBreakHint(unit string) string
	Finish() string
	TournamentStatsHint() string

	// Errors of these pages.
	ErrLogin() string
	ErrTooManyLogins() string
	ErrUsernameTaken() string
	ErrRegisterInput() string
	ErrInvalidEmail() string
	ErrCaptcha() string
	ErrEmailTaken() string
	ErrEnterName() string
	ErrGuestNameTaken() string
	ErrGameNotStarted() string
	ErrChoosePlayer() string
	ErrTournamentPlayers(min, max int) string
	ErrUnknownUser(username string) string
	ErrNotFriend(username string) string
	ErrInviteSelf() string
	ErrAlreadyFriends() string
	ErrFriendSelf() string
	ErrMatchNotStartable() string
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

// languageFlag is the flag a language is shown with.
func languageFlag(l i18n.Lang) string {
	switch l {
	case i18n.German:
		return "🇩🇪"
	}
	return "🇬🇧"
}
