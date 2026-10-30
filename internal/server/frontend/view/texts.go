package view

import (
	"context"
	"time"

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
	Offline() string
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
	// FormAverage and FormHitRate show a player's form in the list of open games (the
	// three-dart average in X01, the hit rate in around the clock), NewPlayer one with
	// too few games for it.
	FormAverage(average string) string
	FormHitRate(rate string) string
	NewPlayer() string
	JoinLobby() string
	LobbiesHint() string
	LobbiesShown(shown, total int) string
	NoMatchingGames() string
	WaitingMinutes(n int) string
	WaitingHours(n int) string
	WaitingDays(n int) string
	GameMode() string
	FilterAny() string
	FilterYes() string
	FilterNo() string
	FilterReset() string
	Strangers() string
	AllowStrangers() string
	StrangerSeats() string
	StrangersHint() string
	WaitingForSeats(n int) string
	ErrNoSeat() string
	ErrOpenWithoutTurnLimit() string
	LandingLead() string
	LandingFeatureTitle(i int) string
	LandingFeatureText(i int) string
	ProjectPage() string
	ProjectPageURL() string // the project page in this language
	ProjectPageHint() string
	About() string
	Imprint() string
	Privacy() string
	PageHint() string
	// NoImprint and NoPrivacy stand on the page while it has no text.
	NoImprint() string
	NoPrivacy() string
	SavePage() string
	ShowPage() string
	Saved() string
	ErrPageTooLong(max int) string
	AdminTitle() string
	AdminHint() string
	AdminLinkHint() string
	OpenAdmin() string
	RegistrationTitle() string
	// RegistrationIsOpen and RegistrationIsClosed say on the admin page whether anyone may
	// create an account.
	RegistrationIsOpen() string
	RegistrationIsClosed() string
	OpenRegistration() string
	CloseRegistration() string
	AdminsTitle() string
	AdminsHint() string
	MakeAdmin() string
	RemoveAdmin() string
	RemoveAdminConfirm(username string) string
	// CaptchaOn, CaptchaTurnedOff and CaptchaOff say on the admin page whether registering
	// needs the captcha: on, turned off by an admin, or not possible without the keys from
	// the server's start options. TurnCaptchaOn and TurnCaptchaOff are its button.
	// The list of accounts on the admin page: AccountsHint names how many it shows without a
	// search, MoreAccounts says that there are more than that.
	AccountsTitle() string
	AccountsHint(limit int) string
	SearchAccounts() string
	Search() string
	MoreAccounts(limit int) string
	NoAccounts() string
	Registered(date string) string
	// LastSeen is when the account last made a request or had a live page open.
	LastSeen(date string) string
	NeverSeen() string
	AdminBadge() string
	DisabledBadge() string
	DisableAccount() string
	EnableAccount() string
	DeleteOtherAccountConfirm(username string) string
	ErrAdminAccount() string
	ErrAccountDisabled() string
	// Date is a day in the language's usual way, e.g. Oct 30, 2026 or 30.10.2026.
	Date(t time.Time) string
	CaptchaOn() string
	CaptchaTurnedOff() string
	CaptchaOff() string
	TurnCaptchaOn() string
	TurnCaptchaOff() string
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
	ChangePasswordTitle() string
	ChangePasswordHint() string
	CurrentPassword() string
	NewPassword() string
	ChangePasswordButton() string
	PasswordChanged() string
	ErrInvalidPassword() string
	ErrTooManyGuests(max int) string
	ErrTooManyFriends(max int) string
	ErrLastAdmin() string
	LeaveGame() string
	LeaveGameConfirm() string
	PlayerLeft() string
	YouLeft() string
	// CamerasMissing warns that only watching of the cameras at a board detect darts.
	CamerasMissing(board string, watching, cameras int) string
	// SmallFrames warns that a camera at a board delivers only small frames.
	SmallFrames(board, camera string, width, height int) string
	// GridMismatch warns that the calibration of a camera at a board doesn't fit anymore.
	GridMismatch(board, camera string) string
	// BoardWarning is a warning about a board that this server doesn't know.
	BoardWarning(board string) string
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
	NewVersion(version string) string
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
	// Quick match and the strength filter in the list of open games.
	QuickMatch() string
	QuickMatchHint() string
	FindOpponent() string
	QuickMatchWaiting() string
	Strength() string
	AboutMine() string
	TurnTime(seconds int) string
	TurnLimitHint(skips int) string
	TurnTimeLeft() string
	Players() string
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
	CalibrationHint() string
	DownloadCam() string
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
	Incognito() string
	IncognitoHint() string
	AppearOffline() string
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
	PresenceOnline() string
	PresencePlaying() string
	PresenceOffline() string

	// Statistics.
	StatsHint() string
	NoFinishedGames() string
	Heatmap() string
	AllModes() string
	RatePlayers() string
	RatePlayersHint() string
	ThumbUp() string
	ThumbDown() string
	RatingsReceived() string
	LeftEarly() string
	BoardRequired() string
	BoardOnlyOption() string
	BoardOnlyHint() string
	NeedsBoard() string
	ErrBoardRequired() string
	ErrBoardBusy() string
	ErrPlayerBusy(name string) string
	ErrStarted() string
	LobbyHint() string
	FirstPlayerHint() string
	StartsWhenFull() string
	StartTournament() string
	AddGuest() string
	Invite() string
	RemoveFromLobby() string
	Save() string
	Guests() string
	NoGuests() string
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
