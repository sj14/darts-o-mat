package view

import (
	"math"
	"strconv"
	"strings"
)

// textsDE are the German texts. Whatever isn't defined here comes from textsEN.
type textsDE struct{ textsEN }

func (textsDE) Decimal(f float64) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', 1, 64), ".", ",", 1)
}
func (textsDE) Percent(ratio float64) string { return strconv.Itoa(int(math.Round(ratio*100))) + " %" }

func (textsDE) NavStats() string        { return "Statistik" }
func (textsDE) NavFriends() string      { return "Freunde" }
func (textsDE) Logout() string          { return "Abmelden" }
func (textsDE) Language() string        { return "Sprache" }
func (textsDE) LanguageBrowser() string { return "wie Browser" }
func (textsDE) SoundOn() string         { return "🔊 Ton an" }
func (textsDE) SoundOff() string        { return "🔇 Ton aus" }
func (textsDE) SoundBlocked() string    { return "🔈 Für Ton klicken" }

func (textsDE) WithBull() string        { return " mit Bull" }
func (textsDE) MultiplierSkips() string { return ", Doppel/Triple springen" }
func (textsDE) FirstToSets(sets, legs int) string {
	return "First to " + strconv.Itoa(sets) + " Sets à " + strconv.Itoa(legs) + " " + plural(legs, "Leg", "Legs")
}

func (textsDE) ToTournament() string  { return "🏆 Zum Turnier" }
func (textsDE) BullOff() string       { return "Ausbullen" }
func (textsDE) RoundN(n int) string   { return "Runde " + strconv.Itoa(n) }
func (textsDE) InvitedToGame() string { return "Du bist zu diesem Spiel eingeladen." }
func (textsDE) Accept() string        { return "Zusagen" }
func (textsDE) Decline() string       { return "Absagen" }
func (textsDE) GameStartsWhenAccepted() string {
	return "Das Spiel beginnt, sobald alle Eingeladenen zugesagt haben."
}
func (textsDE) GuestMark() string { return "(Gast)" }
func (textsDE) Joined() string    { return "dabei" }
func (textsDE) Accepted() string  { return "zugesagt" }
func (textsDE) Invited() string   { return "eingeladen" }
func (textsDE) Uninvite() string  { return "Ausladen" }
func (textsDE) AbortGame() string { return "Spiel abbrechen" }
func (textsDE) AbortGameConfirm() string {
	return "Spiel wirklich abbrechen? Es zählt dann nicht in die Statistik."
}

func (textsDE) Wins(name string) string    { return "🏆 " + name + " gewinnt!" }
func (textsDE) NewGame() string            { return "Neues Spiel" }
func (textsDE) GameAborted() string        { return "Spiel abgebrochen" }
func (textsDE) CheckoutSuggestion() string { return "Vorschlag zum Auschecken" }
func (textsDE) PullDarts() string          { return "Pfeile ziehen" }
func (textsDE) ThenSet(n int) string       { return "Danach beginnt Set " + strconv.Itoa(n) + "." }
func (textsDE) ThenLeg(n int) string       { return "Danach beginnt Leg " + strconv.Itoa(n) + "." }
func (textsDE) ThenPlayer(name string) string {
	return "Danach ist " + name + " dran."
}
func (textsDE) CameraSeesTakeout() string { return "Die Kamera erkennt das Herausziehen von selbst." }
func (textsDE) DartsPulled() string       { return "Pfeile gezogen" }
func (textsDE) UndoLastDart() string      { return "↶ Letzten Dart zurücknehmen" }
func (textsDE) Starts(name string) string { return name + " beginnt!" }
func (textsDE) SetTo(name string) string  { return "Set an " + name + "!" }
func (textsDE) LegTo(name string) string  { return "Leg an " + name + "!" }
func (textsDE) Bust() string              { return "Überworfen!" }
func (textsDE) Busted() string            { return "überworfen" }
func (textsDE) Win() string               { return "Sieg" }
func (textsDE) ToThrow() string           { return "am Zug" }
func (textsDE) Next() string              { return "als Nächstes" }
func (textsDE) SetsWon(n int) string {
	return strconv.Itoa(n) + " " + plural(n, "Set", "Sets") + " gewonnen"
}
func (textsDE) LegsWon(n int) string {
	return strconv.Itoa(n) + " " + plural(n, "Leg", "Legs") + " gewonnen"
}
func (textsDE) Remaining() string   { return "Rest" }
func (textsDE) Average3() string    { return "Ø 3 Darts" }
func (textsDE) AverageHint() string { return "Punkte pro 3 Darts" }
func (textsDE) Target() string      { return "Ziel" }
func (textsDE) HitRate() string     { return "Trefferquote" }

func (textsDE) Multiplier() string  { return "Multiplikator" }
func (textsDE) Double() string      { return "Doppel" }
func (textsDE) Miss() string        { return "Daneben" }
func (textsDE) Hit() string         { return "Treffer" }
func (textsDE) OtherNumber() string { return "Andere Zahl eintragen" }
func (textsDE) Correct() string     { return "Korrigieren" }
func (textsDE) CorrectDart() string { return "Dart korrigieren:" }
func (textsDE) DartBy(dart, name string) string {
	return dart + " von " + name
}
func (textsDE) CorrectHint() string {
	return "Wähle, was stattdessen getroffen wurde; Punkte, Rest und Statistik rechnen sich neu."
}
func (textsDE) Cancel() string { return "Abbrechen" }
func (textsDE) BullOffRule(name string) string {
	return name + " wirft auf das Bull. Wer am nächsten dran ist, beginnt; bei Gleichstand werfen die Gleichen nochmal."
}
func (textsDE) BullOffAgain() string { return "Stechen" }
func (textsDE) BullOffTie() string   { return "Gleichstand! Wer war näher dran? Sonst werft nochmal." }
func (textsDE) WasCloser(name string) string {
	return name + " war näher"
}

func (textsDE) History() string    { return "Verlauf" }
func (textsDE) Statistics() string { return "Statistik" }
func (textsDE) Player() string     { return "Spieler" }
func (textsDE) Round() string      { return "Runde" }
func (textsDE) Points() string     { return "Punkte" }
func (textsDE) Highest() string    { return "Höchste" }
func (textsDE) Hits() string       { return "Treffer" }
func (textsDE) Finished() string   { return "✓ fertig" }

func (textsDE) ErrInput() string       { return "Ungültige Eingabe." }
func (textsDE) ErrTripleBull() string  { return "Es gibt kein Triple-Bull." }
func (textsDE) ErrNoSuchField() string { return "Diesen Treffer gibt es nicht." }
func (textsDE) ErrChangesThrowers() string {
	return "Diese Korrektur würde ändern, wer die folgenden Darts geworfen hat (z. B. Überworfen mitten in der Aufnahme). Nimm stattdessen die späteren Darts zurück und trage sie neu ein."
}
func (textsDE) ErrEndsEarlier() string {
	return "Mit dieser Korrektur wäre das Spiel schon vor späteren Darts vorbei. Nimm stattdessen die späteren Darts zurück."
}
func (textsDE) ErrNoBullOffTie() string { return "Gerade ist beim Ausbullen niemand gleichauf." }
func (textsDE) ErrNoInvitation() string { return "Diese Einladung gibt es nicht mehr." }
func (textsDE) ErrGameOver() string     { return "Das Spiel ist schon vorbei." }
func (textsDE) ErrPullDartsFirst() string {
	return "Erst die Pfeile ziehen, dann ist der Nächste dran."
}
func (textsDE) ErrNotReady() string      { return "Es haben noch nicht alle Eingeladenen zugesagt." }
func (textsDE) ErrAborted() string       { return "Das Spiel wurde abgebrochen." }
func (textsDE) ErrNothingToUndo() string { return "Es gibt keinen Dart zum Zurücknehmen." }
func (textsDE) ErrForbidden() string     { return "Das darfst du in diesem Spiel nicht." }
