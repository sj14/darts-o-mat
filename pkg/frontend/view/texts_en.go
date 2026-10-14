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

func (textsEN) NavStats() string        { return "Statistics" }
func (textsEN) NavFriends() string      { return "Friends" }
func (textsEN) NavBoards() string       { return "Boards" }
func (textsEN) Logout() string          { return "Log out" }
func (textsEN) Language() string        { return "Language" }
func (textsEN) LanguageBrowser() string { return "Browser" }
func (textsEN) SoundOn() string         { return "🔊 Sound on" }
func (textsEN) SoundOff() string        { return "🔇 Sound off" }
func (textsEN) SoundBlocked() string    { return "🔈 Click for sound" }

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
func (textsEN) GuestMark() string { return "(guest)" }
func (textsEN) Joined() string    { return "in" }
func (textsEN) Accepted() string  { return "accepted" }
func (textsEN) Invited() string   { return "invited" }
func (textsEN) Uninvite() string  { return "Withdraw" }
func (textsEN) AbortGame() string { return "Abort game" }
func (textsEN) AbortGameConfirm() string {
	return "Really abort the game? It won't count in the statistics."
}

func (textsEN) Wins(name string) string    { return "🏆 " + name + " wins!" }
func (textsEN) NewGame() string            { return "New game" }
func (textsEN) GameAborted() string        { return "Game aborted" }
func (textsEN) CheckoutSuggestion() string { return "Suggested checkout" }
func (textsEN) Checkout() string           { return "Checkout" }
func (textsEN) PullDarts() string          { return "Pull out the darts" }
func (textsEN) ThenSet(n int) string       { return "Then set " + strconv.Itoa(n) + " starts." }
func (textsEN) ThenLeg(n int) string       { return "Then leg " + strconv.Itoa(n) + " starts." }
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
