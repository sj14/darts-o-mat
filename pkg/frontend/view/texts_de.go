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

func (textsDE) NavStats() string     { return "Statistik" }
func (textsDE) NavFriends() string   { return "Freunde" }
func (textsDE) Logout() string       { return "Abmelden" }
func (textsDE) Language() string     { return "Sprache" }
func (textsDE) SoundOn() string      { return "🔊 Ton an" }
func (textsDE) SoundOff() string     { return "🔇 Ton aus" }
func (textsDE) SoundBlocked() string { return "🔈 Für Ton klicken" }

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

func (textsDE) WinsGame(name string) string { return "🏆 " + name + " gewinnt!" }
func (textsDE) NewGame() string             { return "Neues Spiel" }
func (textsDE) GameAborted() string         { return "Spiel abgebrochen" }
func (textsDE) CheckoutSuggestion() string  { return "Vorschlag zum Auschecken" }
func (textsDE) PullDarts() string           { return "Pfeile ziehen" }
func (textsDE) ThenSet(n int) string        { return "Danach beginnt Set " + strconv.Itoa(n) + "." }
func (textsDE) ThenLeg(n int) string        { return "Danach beginnt Leg " + strconv.Itoa(n) + "." }
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

func (textsDE) HomeTitle() string { return "Start" }
func (textsDE) FriendRequests(n int) string {
	if n == 1 {
		return "Du hast eine neue Freundschaftsanfrage."
	}
	return "Du hast " + strconv.Itoa(n) + " neue Freundschaftsanfragen."
}
func (textsDE) Show() string                    { return "Ansehen" }
func (textsDE) Tournaments() string             { return "Turniere" }
func (textsDE) Invitations() string             { return "Einladungen" }
func (textsDE) TournamentInvitations() string   { return "Turnier-Einladungen" }
func (textsDE) WithPlayers(names string) string { return "mit " + names }
func (textsDE) PlayerCount(n int) string        { return strconv.Itoa(n) + " Spieler" }
func (textsDE) Guests() string                  { return "Gastspieler" }
func (textsDE) GuestsHint() string              { return "Für Mitspieler ohne eigenen Account. Nur du siehst sie." }
func (textsDE) GuestName() string               { return "Name des Gastspielers" }
func (textsDE) Add() string                     { return "Anlegen" }
func (textsDE) RecentGames() string             { return "Letzte Spiele" }
func (textsDE) StatusAborted() string           { return "abgebrochen" }
func (textsDE) StatusWaiting() string           { return "wartet" }
func (textsDE) StatusOver() string              { return "beendet" }
func (textsDE) StatusRunning() string           { return "läuft" }
func (textsDE) GameFormat() string              { return "Spielform" }
func (textsDE) SingleGame() string              { return "Einzelspiel" }
func (textsDE) Knockout() string                { return "K.-o.-Turnier" }
func (textsDE) RoundRobin() string              { return "Jeder gegen jeden" }
func (textsDE) TournamentHint(min, max int) string {
	return "Ein Turnier braucht " + strconv.Itoa(min) + " bis " + strconv.Itoa(max) + " Spieler; die Reihenfolge wird ausgelost, jedes Spiel hat die Einstellungen hier."
}
func (textsDE) OneLeg() string { return "1 Leg" }
func (textsDE) LegsHint() string {
	return "Wer auscheckt, gewinnt das Leg sofort; jedes Leg beginnt der Nächste. Mit Sets gilt das pro Set."
}
func (textsDE) NoSets() string { return "Ohne Sets" }
func (textsDE) SetsHint() string {
	return "Wer die Legs eines Sets zuerst gewinnt, gewinnt das Set; jedes Set beginnt der Nächste."
}
func (textsDE) WhoStarts() string { return "Wer beginnt" }
func (textsDE) BullOffOptionHint() string {
	return "– wer am nächsten am Bull ist; mit Kamera auf den Millimeter"
}
func (textsDE) Players() string         { return "Mitspieler" }
func (textsDE) Me(name string) string   { return "Ich (" + name + ")" }
func (textsDE) InviteFriends() string   { return "Freunde einladen" }
func (textsDE) NoFriendsBefore() string { return "Füge unter " }
func (textsDE) NoFriendsAfter() string  { return " andere Accounts hinzu, um sie einzuladen." }
func (textsDE) NoBoard() string         { return "ohne" }
func (textsDE) X01Description() string  { return "Von 301, 501 oder 701 auf genau 0 runterspielen." }
func (textsDE) AroundTheClockDescription() string {
	return "Die Zahlen 1 bis 20 der Reihe nach treffen."
}
func (textsDE) StartScore() string           { return "Startpunkte" }
func (textsDE) CheckOutWith() string         { return "Auschecken mit" }
func (textsDE) AnyDart() string              { return "beliebigem Dart" }
func (textsDE) DoubleOrTriple() string       { return "Doppel oder Triple" }
func (textsDE) StartMode(mode string) string { return mode + " starten" }
func (textsDE) Rules() string                { return "Regeln" }
func (textsDE) BullAtTheEnd() string         { return "Zum Schluss das Bull treffen" }
func (textsDE) SkipsRule() string            { return "Doppel springt 2 Felder, Triple 3 Felder weiter" }
func (textsDE) ChooseMode() string           { return "Wähle zuerst einen Spielmodus." }

func (textsDE) BoardsHint() string {
	return "Deine Dartboards, z. B. eins im Keller und eins im Vereinsheim. Beim Spielstart wählst du, auf welchem Board gespielt wird; dessen Kameras tragen die Würfe dann automatisch ein."
}
func (textsDE) CameraTokenFor(board string) string { return "Kamera-Token für „" + board + "“" }
func (textsDE) TokenShownOnce() string {
	return "Er wird nur jetzt angezeigt. Trage beides in darts-o-cam am Rechner mit den Kameras unter „Einstellungen“ ein:"
}
func (textsDE) CameraToken() string { return "Kamera-Token" }
func (textsDE) CalibrationHintBefore() string {
	return "Die Kalibrierung liegt auf dem Rechner mit den Kameras, nicht auf diesem Server. Noch keine? Starte dort "
}
func (textsDE) CalibrationHintMiddle() string {
	return "; die Oberfläche öffnet sich im Browser ("
}
func (textsDE) CameraTokenActive() string { return "Kamera-Token aktiv" }
func (textsDE) CreateCameraToken() string { return "Kamera-Token erzeugen" }
func (textsDE) NewCameraToken() string    { return "Neuen Kamera-Token erzeugen" }
func (textsDE) Delete() string            { return "Löschen" }
func (textsDE) BoardNameExample() string  { return "Name, z. B. Keller" }
func (textsDE) BoardName() string         { return "Name des Boards" }

func (textsDE) Login() string        { return "Anmelden" }
func (textsDE) Register() string     { return "Registrieren" }
func (textsDE) Username() string     { return "Benutzername" }
func (textsDE) Password() string     { return "Passwort" }
func (textsDE) NoAccountYet() string { return "Noch kein Account?" }
func (textsDE) UsernameHint() string {
	return "3 bis 32 Zeichen: Buchstaben, Ziffern, Punkt, Bindestrich oder Unterstrich. Damit laden dich andere zu Spielen ein."
}
func (textsDE) DisplayName() string { return "Anzeigename" }
func (textsDE) DisplayNameHint() string {
	return "So heißt du im Spiel. Leer lassen, um den Benutzernamen zu verwenden."
}
func (textsDE) PasswordHint() string  { return "Mindestens 8 Zeichen." }
func (textsDE) CreateAccount() string { return "Account anlegen" }
func (textsDE) RegistrationClosed() string {
	return "Die Registrierung ist auf diesem Server abgeschaltet."
}
func (textsDE) AlreadyRegistered() string { return "Schon registriert?" }

func (textsDE) FriendsHint() string {
	return "Nur Freunde kannst du zu Spielen einladen. Eine Freundschaft gilt, sobald die andere Seite deine Anfrage annimmt."
}
func (textsDE) SendRequest() string     { return "Anfrage senden" }
func (textsDE) RequestsToYou() string   { return "Anfragen an dich" }
func (textsDE) AcceptRequest() string   { return "Annehmen" }
func (textsDE) DeclineRequest() string  { return "Ablehnen" }
func (textsDE) YourFriends() string     { return "Deine Freunde" }
func (textsDE) NoFriendsYet() string    { return "Noch keine Freunde." }
func (textsDE) Remove() string          { return "Entfernen" }
func (textsDE) SentRequests() string    { return "Gesendete Anfragen" }
func (textsDE) WithdrawRequest() string { return "Zurückziehen" }

func (textsDE) StatsHint() string {
	return "Über alle beendeten Spiele und Turniere von dir und deinen Gastspielern."
}
func (textsDE) NoFinishedGames() string { return "Noch keine beendeten Spiele." }
func (textsDE) Count180() string        { return "180er" }
func (textsDE) GamesWinsLegs() string   { return "Spiele / Siege / Legs" }
func (textsDE) SetsWonLabel() string    { return "Sets gewonnen" }
func (textsDE) TournamentsWon() string  { return "Turniere / gewonnen" }
func (textsDE) HighestFinish() string   { return "Höchstes Finish" }
func (textsDE) BestLeg() string         { return "Bestes Leg" }
func (textsDE) NDarts(n int) string     { return strconv.Itoa(n) + " Darts" }
func (textsDE) Games() string           { return "Spiele" }
func (textsDE) DartsHits() string       { return "Darts / Treffer" }

func (textsDE) InvitedToTournament() string {
	return "Du bist zu diesem Turnier eingeladen. Die Spiele beginnen, sobald alle zugesagt haben."
}
func (textsDE) WaitingFor(names string) string {
	return "Wartet auf die Zusage von " + names + ". Danach können die Spiele beginnen."
}
func (textsDE) WinsTournament(name string) string { return "🏆 " + name + " gewinnt das Turnier!" }
func (textsDE) AbortTournament() string           { return "Turnier abbrechen" }
func (textsDE) AbortTournamentConfirm() string    { return "Turnier wirklich abbrechen?" }
func (textsDE) Bracket() string                   { return "Turnierbaum" }
func (t textsDE) RoundName(rounds, round int) string {
	switch rounds - round {
	case 0:
		return "Finale"
	case 1:
		return "Halbfinale"
	case 2:
		return "Viertelfinale"
	case 3:
		return "Achtelfinale"
	case 4:
		return "Sechzehntelfinale"
	}
	return t.RoundN(round)
}
func (textsDE) Bye() string           { return "Freilos" }
func (textsDE) Open() string          { return "offen" }
func (textsDE) StartGame() string     { return "Spiel starten" }
func (textsDE) RunningToGame() string { return "läuft – zum Spiel" }
func (textsDE) ToGame() string        { return "zum Spiel" }
func (textsDE) Table() string         { return "Tabelle" }
func (textsDE) Wins() string          { return "Siege" }
func (textsDE) TieBreakHint(unit string) string {
	return "Bei gleich vielen Siegen entscheidet die Differenz der gewonnenen und verlorenen " + unit + ", dann mehr gewonnene, dann die Auslosung."
}
func (textsDE) TournamentStatsHint() string { return "Über die beendeten Spiele dieses Turniers." }

func (textsDE) ErrLogin() string         { return "Benutzername oder Passwort ist falsch." }
func (textsDE) ErrUsernameTaken() string { return "Dieser Benutzername ist schon vergeben." }
func (textsDE) ErrRegisterInput() string {
	return "Bitte Benutzername (3 bis 32 Zeichen: Buchstaben, Ziffern, . - _) und Passwort (mindestens 8 Zeichen) prüfen."
}
func (textsDE) ErrEnterName() string { return "Bitte einen Namen eingeben." }
func (textsDE) ErrGameNotStarted() string {
	return "Das Spiel konnte nicht gestartet werden. Bitte die Einstellungen prüfen."
}
func (textsDE) ErrChoosePlayer() string {
	return "Bitte mindestens einen Mitspieler auswählen oder einladen."
}
func (textsDE) ErrTournamentPlayers(min, max int) string {
	return "Ein Turnier braucht " + strconv.Itoa(min) + " bis " + strconv.Itoa(max) + " Spieler: dich, Gäste und eingeladene Freunde."
}
func (textsDE) ErrUnknownUser(username string) string {
	return "Einen Account mit dem Benutzernamen „" + username + "“ gibt es nicht."
}
func (textsDE) ErrNotFriend(username string) string {
	return "„" + username + "“ ist nicht mit dir befreundet und kann deshalb nicht eingeladen werden."
}
func (textsDE) ErrInviteSelf() string {
	return "Du kannst dich nicht selbst einladen. Kreuze dich stattdessen bei den Mitspielern an."
}
func (textsDE) ErrAlreadyFriends() string {
	return "Ihr seid schon befreundet oder die Anfrage läuft bereits."
}
func (textsDE) ErrFriendSelf() string { return "Du kannst dich nicht selbst als Freund hinzufügen." }
func (textsDE) ErrMatchNotStartable() string {
	return "Dieses Spiel kann gerade nicht gestartet werden."
}
func (textsDE) ErrAbortTournament() string {
	return "Nur wer das Turnier angelegt hat, kann es abbrechen, und nur solange es läuft."
}
