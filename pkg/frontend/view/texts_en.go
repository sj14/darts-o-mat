package view

import (
	"math"
	"strconv"
)

// textsEN are the English texts, the default language: complete, so every other
// language falls back to them.
type textsEN struct{}

func (textsEN) Decimal(f float64) string     { return strconv.FormatFloat(f, 'f', 1, 64) }
func (textsEN) Percent(ratio float64) string { return strconv.Itoa(int(math.Round(ratio*100))) + "%" }

func (textsEN) NavStats() string    { return "Statistics" }
func (textsEN) NavFriends() string  { return "Friends" }
func (textsEN) NavBoards() string   { return "Boards" }
func (textsEN) NavSettings() string { return "Settings" }
func (textsEN) AccentColor() string { return "Accent color" }
func (textsEN) AccentColorHint() string {
	return "Your color on the game page, e.g. as the frame of the card when it is your turn. Without a choice it comes from your name."
}
func (textsEN) ColorAuto() string { return "From my name" }
func (textsEN) ColorName(color string) string {
	if name, ok := map[string]string{"green": "Green", "red": "Red", "sky": "Sky blue", "violet": "Violet", "pink": "Pink", "orange": "Orange", "teal": "Teal", "indigo": "Indigo", "fuchsia": "Fuchsia", "cyan": "Cyan"}[color]; ok {
		return name
	}
	return color
}
func (textsEN) Logout() string       { return "Log out" }
func (textsEN) Language() string     { return "Language" }
func (textsEN) SoundOn() string      { return "🔊 Sound on" }
func (textsEN) SoundOff() string     { return "🔇 Sound off" }
func (textsEN) SoundBlocked() string { return "🔈 Click for sound" }

func (textsEN) AroundTheClock() string  { return "Around the Clock" }
func (textsEN) WithBull() string        { return " with bull" }
func (textsEN) MultiplierSkips() string { return ", doubles/triples skip" }
func (textsEN) FirstToSets(sets, legs int) string {
	return "First to " + strconv.Itoa(sets) + " sets of " + strconv.Itoa(legs) + " " + plural(legs, "leg", "legs")
}

func (textsEN) ToTournament() string  { return "🏆 To the tournament" }
func (textsEN) BullOff() string       { return "Bull-off" }
func (textsEN) SetN(n int) string     { return "Set " + strconv.Itoa(n) }
func (textsEN) LegN(n int) string     { return "Leg " + strconv.Itoa(n) }
func (textsEN) RoundN(n int) string   { return "Round " + strconv.Itoa(n) }
func (textsEN) InvitedToGame() string { return "You're invited to this game." }
func (textsEN) Accept() string        { return "Accept" }
func (textsEN) Decline() string       { return "Decline" }
func (textsEN) GameStartsWhenAccepted() string {
	return "The game starts as soon as everyone invited has accepted."
}
func (textsEN) GuestMark() string  { return "(guest)" }
func (textsEN) Joined() string     { return "in" }
func (textsEN) Accepted() string   { return "accepted" }
func (textsEN) Invited() string    { return "invited" }
func (textsEN) Uninvite() string   { return "Withdraw" }
func (textsEN) AbortGame() string  { return "Abort game" }
func (textsEN) BoardInUse() string { return "in use, the other game loses it" }
func (textsEN) LandingLead() string {
	return "A free, open-source darts scorer that you host yourself. Enter your throws or let cameras detect them – the score is counted for you."
}

func (textsEN) LandingFeatureTitle(i int) string {
	return [...]string{"Self-hosted", "Free & open source", "Auto-scoring", "Play together"}[i%4]
}

func (textsEN) LandingFeatureText(i int) string {
	return [...]string{
		"One program or one container, with SQLite for the living room or PostgreSQL for more. Your games and data stay with you.",
		"Licensed under the AGPL-3.0. No subscription, no account at somebody else’s service.",
		"darts-o-cam detects the darts with cheap cameras at the board and enters them into the running game.",
		"X01 and Around the Clock, legs and sets, knockout and round-robin tournaments, friends, guests and statistics.",
	}[i%4]
}

func (textsEN) OpenGames() string   { return "Open games" }
func (textsEN) NoOpenGames() string { return "No game is looking for players right now." }
func (textsEN) PlayersOfSeats(players, seats int) string {
	return strconv.Itoa(players) + " of " + strconv.Itoa(seats) + " players"
}
func (textsEN) JoinLobby() string      { return "Join" }
func (textsEN) Strangers() string      { return "Strangers" }
func (textsEN) AllowStrangers() string { return "Let strangers join" }
func (textsEN) StrangerSeats() string  { return "Players in total" }
func (textsEN) StrangersHint() string {
	return "The game shows up in the list of open games for 30 minutes and starts once all seats are taken. A tournament needs at least 3 players."
}
func (textsEN) WaitingForSeats(n int) string {
	if n == 1 {
		return "Waiting for 1 more player."
	}
	return "Waiting for " + strconv.Itoa(n) + " more players."
}
func (textsEN) ErrNoSeat() string     { return "There is no free seat anymore." }
func (textsEN) DeletedPlayer() string { return "Deleted player" }
func (textsEN) DeleteGuest() string   { return "Delete" }
func (textsEN) DeleteGuestConfirm(name string) string {
	return "Delete the guest " + name + "? Their games stay, with them shown as a deleted player."
}
func (textsEN) ErrGuestBusy() string {
	return "This guest takes part in a running game or tournament. Finish or abort it first."
}
func (textsEN) DeleteAccountTitle() string { return "Delete account" }
func (textsEN) DeleteAccountHint() string {
	return "Deletes your login, your friendships, your boards and your guests. Your own running games and tournaments are aborted, in those of others you leave. Finished games stay for the others, with you shown as a deleted player. This can't be undone."
}
func (textsEN) DeleteAccountButton() string { return "Delete my account" }
func (textsEN) DeleteAccountConfirm() string {
	return "Really delete your account? This can't be undone."
}
func (textsEN) ErrWrongPassword() string { return "The password is wrong." }
func (textsEN) LeaveGame() string        { return "Leave game" }
func (textsEN) LeaveGameConfirm() string {
	return "Really leave the game? The others play on without you."
}
func (textsEN) PlayerLeft() string { return "left" }
func (textsEN) YouLeft() string    { return "You left this game. You can still look at it." }
func (textsEN) ErrCantLeave() string {
	return "You can't leave the game now, e.g. not while bulling off."
}
func (textsEN) EnterDart() string   { return "Enter a dart" }
func (textsEN) HideKeypad() string  { return "Hide keypad" }
func (textsEN) PlayOnBoard() string { return "Play on this board" }
func (textsEN) BoardInUseConfirm() string {
	return "Another game is on this board. Move the board to this game? The other game continues without a board."
}
func (textsEN) AbortGameConfirm() string {
	return "Really abort the game? It won't count in the statistics."
}

func (textsEN) WinsGame(name string) string  { return "🏆 " + name + " wins!" }
func (textsEN) Duration(clock string) string { return "Duration " + clock }
func (textsEN) NewGame() string              { return "New game" }
func (textsEN) GameAborted() string          { return "Game aborted" }
func (textsEN) CheckoutSuggestion() string   { return "Suggested checkout" }
func (textsEN) Checkout() string             { return "Checkout" }
func (textsEN) PullDarts() string            { return "Pull out the darts" }
func (textsEN) ThenSet(n int) string         { return "Then set " + strconv.Itoa(n) + " starts." }
func (textsEN) ThenLeg(n int) string         { return "Then leg " + strconv.Itoa(n) + " starts." }
func (textsEN) ThenPlayer(name string) string {
	return "Then it's " + name + "'s turn."
}
func (textsEN) CameraSeesTakeout() string { return "The camera notices the takeout by itself." }
func (textsEN) DartsPulled() string       { return "Darts pulled out" }
func (textsEN) UndoLastDart() string      { return "↶ Undo last dart" }
func (textsEN) Starts(name string) string { return name + " starts!" }
func (textsEN) SetTo(name string) string  { return "Set to " + name + "!" }
func (textsEN) LegTo(name string) string  { return "Leg to " + name + "!" }
func (textsEN) Bust() string              { return "Bust!" }
func (textsEN) Busted() string            { return "bust" }
func (textsEN) Win() string               { return "Win" }
func (textsEN) ToThrow() string           { return "to throw" }
func (textsEN) Next() string              { return "next" }
func (textsEN) SetsWon(n int) string {
	return strconv.Itoa(n) + " " + plural(n, "set", "sets") + " won"
}
func (textsEN) LegsWon(n int) string {
	return strconv.Itoa(n) + " " + plural(n, "leg", "legs") + " won"
}
func (textsEN) Remaining() string   { return "Left" }
func (textsEN) Average3() string    { return "3-dart avg." }
func (textsEN) AverageHint() string { return "Points per 3 darts" }
func (textsEN) Target() string      { return "Target" }
func (textsEN) HitRate() string     { return "Hit rate" }

func (textsEN) Multiplier() string  { return "Multiplier" }
func (textsEN) Single() string      { return "Single" }
func (textsEN) Double() string      { return "Double" }
func (textsEN) Triple() string      { return "Triple" }
func (textsEN) Bull() string        { return "Bull" }
func (textsEN) Bullseye() string    { return "Bullseye" }
func (textsEN) Miss() string        { return "Miss" }
func (textsEN) Hit() string         { return "Hit" }
func (textsEN) OtherNumber() string { return "Enter another number" }
func (textsEN) Correct() string     { return "Correct" }
func (textsEN) CorrectDart() string { return "Correct dart:" }
func (textsEN) DartBy(dart, name string) string {
	return dart + " by " + name
}
func (textsEN) CorrectHint() string {
	return "Choose what was hit instead; points, score left and statistics are recalculated."
}
func (textsEN) Cancel() string { return "Cancel" }
func (textsEN) BullOffRule(name string) string {
	return name + " throws at the bull. Closest starts; on a tie the tied players throw again."
}
func (textsEN) BullOffAgain() string { return "Throw-off" }
func (textsEN) BullOffTie() string   { return "Tie! Who was closer? Otherwise throw again." }
func (textsEN) WasCloser(name string) string {
	return name + " was closer"
}

func (textsEN) History() string    { return "History" }
func (textsEN) Statistics() string { return "Statistics" }
func (textsEN) Player() string     { return "Player" }
func (textsEN) Round() string      { return "Round" }
func (textsEN) Darts() string      { return "Darts" }
func (textsEN) Points() string     { return "Points" }
func (textsEN) Highest() string    { return "Highest" }
func (textsEN) Hits() string       { return "Hits" }
func (textsEN) Finished() string   { return "✓ done" }

func (textsEN) ErrInput() string       { return "Invalid input." }
func (textsEN) ErrTripleBull() string  { return "There is no triple bull." }
func (textsEN) ErrNoSuchField() string { return "That field doesn't exist." }
func (textsEN) ErrChangesThrowers() string {
	return "This correction would change who threw the following darts (e.g. a bust in the middle of a turn). Undo the later darts instead and enter them again."
}
func (textsEN) ErrEndsEarlier() string {
	return "With this correction the game would have been over before later darts. Undo the later darts instead."
}
func (textsEN) ErrNoBullOffTie() string { return "Nobody is tied in the bull-off right now." }
func (textsEN) ErrNoInvitation() string { return "This invitation doesn't exist anymore." }
func (textsEN) ErrGameOver() string     { return "The game is already over." }
func (textsEN) ErrPullDartsFirst() string {
	return "Pull out the darts first, then it's the next player's turn."
}
func (textsEN) ErrNotReady() string      { return "Not everyone invited has accepted yet." }
func (textsEN) ErrAborted() string       { return "The game was aborted." }
func (textsEN) ErrNothingToUndo() string { return "There is no dart to undo." }
func (textsEN) ErrForbidden() string     { return "You aren't allowed to do that in this game." }

func (textsEN) HomeTitle() string { return "Home" }
func (textsEN) FriendRequests(n int) string {
	if n == 1 {
		return "You have a new friend request."
	}
	return "You have " + strconv.Itoa(n) + " new friend requests."
}
func (textsEN) Show() string                    { return "Show" }
func (textsEN) Tournaments() string             { return "Tournaments" }
func (textsEN) Invitations() string             { return "Invitations" }
func (textsEN) TournamentInvitations() string   { return "Tournament invitations" }
func (textsEN) WithPlayers(names string) string { return "with " + names }
func (textsEN) PlayerCount(n int) string {
	return strconv.Itoa(n) + " " + plural(n, "player", "players")
}
func (textsEN) Guests() string { return "Guest players" }
func (textsEN) GuestsHint() string {
	return "For fellow players without an account of their own. Only you see them."
}
func (textsEN) Name() string          { return "Name" }
func (textsEN) GuestName() string     { return "Name of the guest player" }
func (textsEN) Add() string           { return "Add" }
func (textsEN) RecentGames() string   { return "Recent games" }
func (textsEN) StatusAborted() string { return "aborted" }
func (textsEN) StatusWaiting() string { return "waiting" }
func (textsEN) StatusOver() string    { return "over" }
func (textsEN) StatusRunning() string { return "running" }
func (textsEN) GameFormat() string    { return "Format" }
func (textsEN) SingleGame() string    { return "Single game" }
func (textsEN) Knockout() string      { return "Knockout tournament" }
func (textsEN) RoundRobin() string    { return "Round robin" }
func (textsEN) TournamentHint(min, max int) string {
	return "A tournament needs " + strconv.Itoa(min) + " to " + strconv.Itoa(max) + " players; the order is drawn, every game has the settings chosen here."
}
func (textsEN) Legs() string   { return "Legs" }
func (textsEN) Sets() string   { return "Sets" }
func (textsEN) OneLeg() string { return "1 leg" }
func (textsEN) LegsHint() string {
	return "Whoever checks out wins the leg at once; each leg is started by the next player. With sets, this counts per set."
}
func (textsEN) NoSets() string { return "No sets" }
func (textsEN) SetsHint() string {
	return "Whoever wins the legs of a set first wins the set; each set is started by the next player."
}
func (textsEN) WhoStarts() string { return "Who starts" }
func (textsEN) BullOffOptionHint() string {
	return "– closest to the bull; with cameras to the millimetre"
}
func (textsEN) TurnLimit() string   { return "Time per turn" }
func (textsEN) NoTurnLimit() string { return "No limit" }
func (textsEN) TurnTime(seconds int) string {
	switch {
	case seconds == 60:
		return "1 minute"
	case seconds%60 == 0:
		return strconv.Itoa(seconds/60) + " minutes"
	}
	return strconv.Itoa(seconds) + " seconds"
}
func (textsEN) TurnLimitHint(skips int) string {
	return "Whoever takes longer misses the turn, and after " + strconv.Itoa(skips) + " missed turns in a row they leave the game."
}
func (textsEN) TurnTimeLeft() string    { return "Time left for the turn" }
func (textsEN) Players() string         { return "Players" }
func (textsEN) Me(name string) string   { return "Me (" + name + ")" }
func (textsEN) InviteFriends() string   { return "Invite friends" }
func (textsEN) NoFriendsBefore() string { return "Add other accounts under " }
func (textsEN) NoFriendsAfter() string  { return " to invite them." }
func (textsEN) Board() string           { return "Board" }
func (textsEN) NoBoard() string         { return "none" }
func (textsEN) X01Description() string  { return "Play down from 301, 501 or 701 to exactly 0." }
func (textsEN) AroundTheClockDescription() string {
	return "Hit the numbers 1 to 20 in order."
}
func (textsEN) StartScore() string           { return "Starting score" }
func (textsEN) CheckOutWith() string         { return "Check out with" }
func (textsEN) AnyDart() string              { return "any dart" }
func (textsEN) DoubleOrTriple() string       { return "double or triple" }
func (textsEN) StartMode(mode string) string { return "Start " + mode }
func (textsEN) Rules() string                { return "Rules" }
func (textsEN) BullAtTheEnd() string         { return "Hit the bull at the end" }
func (textsEN) SkipsRule() string            { return "A double moves 2 fields on, a triple 3" }
func (textsEN) ChooseMode() string           { return "Choose a game mode first." }

func (textsEN) BoardsHint() string {
	return "Your dartboards, e.g. one in the basement and one at the club. When starting a game you choose the board it's played on; its cameras then enter the throws by themselves."
}
func (textsEN) CameraTokenFor(board string) string { return "Camera token for “" + board + "”" }
func (textsEN) TokenShownOnce() string {
	return "It's shown only now. Enter both in darts-o-cam on the computer with the cameras, under “Settings”:"
}
func (textsEN) Server() string      { return "Server" }
func (textsEN) CameraToken() string { return "Camera token" }
func (textsEN) CalibrationHintBefore() string {
	return "The calibration stays on the computer with the cameras, not on this server. None yet? Start "
}
func (textsEN) CalibrationHintMiddle() string {
	return " there; its interface opens in the browser ("
}
func (textsEN) CameraTokenActive() string { return "Camera token active" }
func (textsEN) CreateCameraToken() string { return "Create camera token" }
func (textsEN) NewCameraToken() string    { return "Create a new camera token" }
func (textsEN) Delete() string            { return "Delete" }
func (textsEN) BoardNameExample() string  { return "Name, e.g. basement" }
func (textsEN) BoardName() string         { return "Name of the board" }

func (textsEN) Login() string        { return "Log in" }
func (textsEN) Register() string     { return "Register" }
func (textsEN) Username() string     { return "Username" }
func (textsEN) Password() string     { return "Password" }
func (textsEN) NoAccountYet() string { return "No account yet?" }
func (textsEN) UsernameHint() string {
	return "3 to 32 characters: letters, digits, dot, hyphen or underscore. Others invite you to games with it."
}
func (textsEN) Email() string         { return "Email" }
func (textsEN) EmailOptional() string { return "Email (optional)" }
func (textsEN) EmailHint() string {
	return "Optional. Nothing is sent to it yet; it is only stored for the account. Leave it empty to remove it."
}
func (textsEN) SaveEmail() string { return "Save" }
func (textsEN) ErrInvalidEmail() string {
	return "Please enter a valid email address, or leave it empty."
}
func (textsEN) ErrEmailTaken() string { return "Another account uses this email address already." }
func (textsEN) DisplayName() string   { return "Display name" }
func (textsEN) DisplayNameHint() string {
	return "Your name in games. Leave it empty to use the username."
}
func (textsEN) PasswordHint() string       { return "At least 8 characters." }
func (textsEN) CreateAccount() string      { return "Create account" }
func (textsEN) RegistrationClosed() string { return "Registration is turned off on this server." }
func (textsEN) AlreadyRegistered() string  { return "Already registered?" }

func (textsEN) FriendsHint() string {
	return "You can only invite friends to games. A friendship counts once the other side accepts your request."
}
func (textsEN) SendRequest() string     { return "Send request" }
func (textsEN) RequestsToYou() string   { return "Requests to you" }
func (textsEN) AcceptRequest() string   { return "Accept" }
func (textsEN) DeclineRequest() string  { return "Decline" }
func (textsEN) YourFriends() string     { return "Your friends" }
func (textsEN) NoFriendsYet() string    { return "No friends yet." }
func (textsEN) Remove() string          { return "Remove" }
func (textsEN) SentRequests() string    { return "Sent requests" }
func (textsEN) WithdrawRequest() string { return "Withdraw" }

func (textsEN) StatsHint() string {
	return "Over all finished games and tournaments of you and your guest players."
}
func (textsEN) NoFinishedGames() string { return "No finished games yet." }
func (textsEN) Count180() string        { return "180s" }
func (textsEN) GamesWinsLegs() string   { return "Games / wins / legs" }
func (textsEN) SetsWonLabel() string    { return "Sets won" }
func (textsEN) TournamentsWon() string  { return "Tournaments / won" }
func (textsEN) HighestFinish() string   { return "Highest finish" }
func (textsEN) BestLeg() string         { return "Best leg" }
func (textsEN) AvgLegTime() string      { return "Average leg time" }
func (textsEN) AvgGameTime() string     { return "Average game time" }
func (textsEN) NDarts(n int) string     { return strconv.Itoa(n) + " darts" }
func (textsEN) Games() string           { return "Games" }
func (textsEN) DartsHits() string       { return "Darts / hits" }

func (textsEN) InvitedToTournament() string {
	return "You're invited to this tournament. The games start as soon as everyone has accepted."
}
func (textsEN) WaitingFor(names string) string {
	return "Waiting for " + names + " to accept. Then the games can start."
}
func (textsEN) WinsTournament(name string) string { return "🏆 " + name + " wins the tournament!" }
func (textsEN) LeaveTournament() string           { return "Leave tournament" }
func (textsEN) LeaveTournamentConfirm() string {
	return "Really leave the tournament? It ends for everyone; finished games stay."
}
func (textsEN) Bracket() string { return "Bracket" }
func (t textsEN) RoundName(rounds, round int) string {
	switch rounds - round {
	case 0:
		return "Final"
	case 1:
		return "Semi-finals"
	case 2:
		return "Quarter-finals"
	case 3:
		return "Round of 16"
	case 4:
		return "Round of 32"
	}
	return t.RoundN(round)
}
func (textsEN) Bye() string           { return "bye" }
func (textsEN) Open() string          { return "open" }
func (textsEN) StartGame() string     { return "Start game" }
func (textsEN) RunningToGame() string { return "running – to the game" }
func (textsEN) ToGame() string        { return "to the game" }
func (textsEN) Table() string         { return "Table" }
func (textsEN) Wins() string          { return "Wins" }
func (textsEN) Difference() string    { return "Diff." }
func (textsEN) TieBreakHint(unit string) string {
	return "With equal wins, the difference of " + unit + " won and lost decides, then more won, then the draw."
}
func (textsEN) Finish() string              { return "Finish" }
func (textsEN) TournamentStatsHint() string { return "Over the finished games of this tournament." }

func (textsEN) ErrLogin() string         { return "Wrong username or password." }
func (textsEN) ErrUsernameTaken() string { return "This username is already taken." }
func (textsEN) ErrRegisterInput() string {
	return "Please check the username (3 to 32 characters: letters, digits, . - _) and the password (at least 8 characters)."
}
func (textsEN) ErrEnterName() string { return "Please enter a name." }
func (textsEN) ErrGuestNameTaken() string {
	return "You already have a player with this name."
}
func (textsEN) ErrGameNotStarted() string {
	return "The game couldn't be started. Please check the settings."
}
func (textsEN) ErrChoosePlayer() string { return "Please choose or invite at least one player." }
func (textsEN) ErrTournamentPlayers(min, max int) string {
	return "A tournament needs " + strconv.Itoa(min) + " to " + strconv.Itoa(max) + " players: you, guests and invited friends."
}
func (textsEN) ErrUnknownUser(username string) string {
	return "There is no account with the username “" + username + "”."
}
func (textsEN) ErrNotFriend(username string) string {
	return "“" + username + "” isn't your friend, so you can't invite them."
}
func (textsEN) ErrInviteSelf() string {
	return "You can't invite yourself. Tick yourself among the players instead."
}
func (textsEN) ErrAlreadyFriends() string {
	return "You're already friends, or the request is pending."
}
func (textsEN) ErrFriendSelf() string        { return "You can't add yourself as a friend." }
func (textsEN) ErrMatchNotStartable() string { return "This game can't be started right now." }
