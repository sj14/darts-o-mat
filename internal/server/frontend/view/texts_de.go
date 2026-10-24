package view

import (
	"math"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/internal/common/buildinfo"
)

// textsDE are the German texts. Whatever isn't defined here comes from textsEN.
type textsDE struct{ textsEN }

func (textsDE) Decimal(f float64) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', 1, 64), ".", ",", 1)
}
func (textsDE) Percent(ratio float64) string { return strconv.Itoa(int(math.Round(ratio*100))) + " %" }

func (textsDE) NavStats() string    { return "Statistik" }
func (textsDE) NavSettings() string { return "Einstellungen" }
func (textsDE) AccentColor() string { return "Akzentfarbe" }
func (textsDE) AccentColorHint() string {
	return "Deine Farbe auf der Spielseite, z. B. als Rahmen der Karte, wenn du dran bist. Ohne Auswahl richtet sie sich nach deinem Namen."
}
func (textsDE) ColorAuto() string { return "Nach meinem Namen" }
func (textsDE) ColorName(color string) string {
	if name, ok := map[string]string{"green": "Grün", "red": "Rot", "sky": "Himmelblau", "violet": "Violett", "pink": "Pink", "orange": "Orange", "teal": "Petrol", "indigo": "Indigo", "fuchsia": "Fuchsia", "cyan": "Cyan"}[color]; ok {
		return name
	}
	return color
}
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
func (textsDE) GuestMark() string     { return "(Gast)" }
func (textsDE) Joined() string        { return "dabei" }
func (textsDE) Accepted() string      { return "zugesagt" }
func (textsDE) Invited() string       { return "eingeladen" }
func (textsDE) Uninvite() string      { return "Ausladen" }
func (textsDE) AbortGame() string     { return "Spiel abbrechen" }
func (textsDE) BoardInUse() string    { return "belegt, das andere Spiel verliert es" }
func (textsDE) LandingLead() string {
	return "Ein freier Open-Source-Darts-Zähler, den du selbst betreibst. Gib deine Würfe ein oder lass Kameras sie erkennen – die Punkte zählt das Programm."
}

func (textsDE) LandingFeatureTitle(i int) string {
	return [...]string{"Selbst gehostet", "Frei & Open Source", "Automatisch zählen", "Zusammen spielen"}[i%4]
}

func (textsDE) LandingFeatureText(i int) string {
	return [...]string{
		"Ein Programm oder ein Container, mit SQLite fürs Wohnzimmer oder PostgreSQL für mehr. Deine Spiele und Daten bleiben bei dir.",
		"Unter der AGPL-3.0 lizenziert. Kein Abo, kein Konto bei einem fremden Dienst.",
		"darts-o-cam erkennt die Darts mit günstigen Kameras am Board und trägt sie ins laufende Spiel ein.",
		"X01 und Around the Clock, Legs und Sets, K.-o.- und Jeder-gegen-jeden-Turniere, Freunde, Gäste und Statistiken.",
	}[i%4]
}

func (textsDE) ProjectPageURL() string { return buildinfo.PageURL + "de/" }
func (textsDE) ProjectPageHint() string {
	return "Auf der Projektseite gibt es darts-o-cam für den Rechner mit den Kameras, den Server und die Anleitungen."
}
func (textsDE) About() string { return "Über darts-o-mat" }

func (textsDE) Imprint() string { return "Impressum" }
func (textsDE) Privacy() string { return "Datenschutz" }
func (textsDE) PageHint() string {
	return "Klartext, Zeilenumbrüche bleiben erhalten. Die Fußzeile jeder Seite verlinkt sie, auch für Besucher ohne Anmeldung; ohne Text sagt die Seite, dass es noch keinen gibt."
}
func (textsDE) NoImprint() string { return "Dieser Server hat noch kein Impressum." }
func (textsDE) NoPrivacy() string { return "Dieser Server hat noch keine Datenschutzerklärung." }
func (textsDE) SavePage() string  { return "Speichern" }
func (textsDE) ShowPage() string  { return "Seite ansehen" }
func (textsDE) Saved() string     { return "Gespeichert." }
func (textsDE) ErrPageTooLong(max int) string {
	return "Der Text ist zu lang, höchstens " + strconv.Itoa(max) + " Zeichen."
}
func (textsDE) AdminTitle() string { return "Administration" }
func (textsDE) AdminHint() string {
	return "Einstellungen dieses Servers für alle, die ihn nutzen. Der zuerst registrierte Account ist Admin; Admins können andere Accounts zu Admins machen."
}
func (textsDE) AdminLinkHint() string {
	return "Du bist Admin dieses Servers: Du öffnest und schließt die Registrierung, schreibst Impressum und Datenschutzerklärung und bestimmst die anderen Admins."
}
func (textsDE) OpenAdmin() string         { return "Administration öffnen" }
func (textsDE) RegistrationTitle() string { return "Registrierung" }
func (textsDE) RegistrationIsOpen() string {
	return "Die Registrierung ist offen: Jeder, der diesen Server erreicht, kann einen Account anlegen."
}
func (textsDE) RegistrationIsClosed() string {
	return "Die Registrierung ist geschlossen: Niemand kann einen neuen Account anlegen."
}
func (textsDE) CaptchaOn() string {
	return "Beim Registrieren wird geprüft, dass ein Mensch es tut (Cloudflare Turnstile), eingestellt in den Startoptionen des Servers."
}
func (textsDE) CaptchaOff() string {
	return "Beim Registrieren wird nicht geprüft, dass ein Mensch es tut; die Startoptionen des Servers können Cloudflare Turnstile einschalten."
}
func (textsDE) OpenRegistration() string {
	return "Registrierung öffnen"
}
func (textsDE) CloseRegistration() string {
	return "Registrierung schließen"
}
func (textsDE) AdminsTitle() string { return "Admins" }
func (textsDE) AdminsHint() string {
	return "Jeder Admin kann alles auf dieser Seite ändern, andere Accounts zu Admins machen und ihnen die Rolle wieder entziehen, auch anderen Admins. Der letzte Admin kann sie nicht abgeben."
}
func (textsDE) MakeAdmin() string   { return "Zum Admin machen" }
func (textsDE) RemoveAdmin() string { return "Admin-Rolle entziehen" }
func (textsDE) RemoveAdminConfirm(username string) string {
	return "„" + username + "“ die Admin-Rolle entziehen?"
}

func (textsDE) OpenGames() string   { return "Offene Spiele" }
func (textsDE) NoOpenGames() string { return "Zurzeit sucht kein Spiel Mitspieler." }
func (textsDE) PlayersOfSeats(players, seats int) string {
	return strconv.Itoa(players) + " von " + strconv.Itoa(seats) + " Spielern"
}
func (textsDE) FormAverage(average string) string { return "Ø " + average }
func (textsDE) FormHitRate(rate string) string    { return rate + " Treffer" }
func (textsDE) NewPlayer() string                 { return "neu" }
func (textsDE) JoinLobby() string                 { return "Beitreten" }
func (textsDE) LobbiesHint() string {
	return "Spiele und Turniere, die Mitspieler suchen. Grenze die Liste auf das passende ein und tritt bei: Es startet, sobald alle Plätze besetzt sind."
}
func (textsDE) LobbiesShown(shown, total int) string {
	return strconv.Itoa(shown) + " von " + strconv.Itoa(total) + " offenen Spielen"
}
func (textsDE) NoMatchingGames() string     { return "Kein offenes Spiel passt zu diesen Filtern." }
func (textsDE) WaitingMinutes(n int) string { return "wartet seit " + strconv.Itoa(n) + " Min." }
func (textsDE) WaitingHours(n int) string   { return "wartet seit " + strconv.Itoa(n) + " Std." }
func (textsDE) WaitingDays(n int) string    { return "wartet seit " + strconv.Itoa(n) + " Tagen" }
func (textsDE) GameMode() string            { return "Spielmodus" }
func (textsDE) FilterAny() string           { return "Egal" }
func (textsDE) FilterYes() string           { return "Ja" }
func (textsDE) FilterNo() string            { return "Nein" }
func (textsDE) FilterReset() string         { return "Filter zurücksetzen" }
func (textsDE) TurnLimited() string         { return "Mit Limit" }
func (textsDE) Strangers() string           { return "Fremde" }
func (textsDE) AllowStrangers() string      { return "Fremde mitspielen lassen" }
func (textsDE) StrangerSeats() string       { return "Spieler insgesamt" }
func (textsDE) StrangersHint() string {
	return "Das Spiel steht 30 Minuten in der Liste der offenen Spiele und startet, sobald alle Plätze besetzt sind. Ein Turnier braucht mindestens 3 Spieler."
}
func (textsDE) WaitingForSeats(n int) string {
	if n == 1 {
		return "Es fehlt noch 1 Spieler."
	}
	return "Es fehlen noch " + strconv.Itoa(n) + " Spieler."
}
func (textsDE) ErrNoSeat() string     { return "Es ist kein Platz mehr frei." }
func (textsDE) DeletedPlayer() string { return "Gelöschter Spieler" }
func (textsDE) DeleteGuest() string   { return "Löschen" }
func (textsDE) DeleteGuestConfirm(name string) string {
	return "Den Gast " + name + " löschen? Seine Spiele bleiben erhalten, er wird dort als gelöschter Spieler angezeigt."
}
func (textsDE) ErrGuestBusy() string {
	return "Dieser Gast nimmt an einem laufenden Spiel oder Turnier teil. Beende oder brich es zuerst ab."
}
func (textsDE) DeleteAccountTitle() string { return "Account löschen" }
func (textsDE) DeleteAccountHint() string {
	return "Löscht deinen Login, deine Freundschaften, deine Boards und deine Gäste. Deine eigenen laufenden Spiele und Turniere werden abgebrochen, bei denen anderer trittst du aus. Beendete Spiele bleiben für die anderen erhalten, du erscheinst dort als gelöschter Spieler. Das lässt sich nicht rückgängig machen."
}
func (textsDE) DeleteAccountButton() string { return "Meinen Account löschen" }
func (textsDE) DeleteAccountConfirm() string {
	return "Deinen Account wirklich löschen? Das lässt sich nicht rückgängig machen."
}
func (textsDE) ErrWrongPassword() string { return "Das Passwort ist falsch." }
func (textsDE) ErrLastAdmin() string {
	return "Du bist der einzige Admin dieses Servers und kannst deinen Account oder die Admin-Rolle deshalb nicht abgeben: Der Server stünde sonst ohne Admin da. Mach zuerst unter „Administration“ einen anderen Account zum Admin."
}
func (textsDE) LeaveGame() string { return "Spiel verlassen" }
func (textsDE) LeaveGameConfirm() string {
	return "Das Spiel wirklich verlassen? Die anderen spielen ohne dich weiter."
}
func (textsDE) PlayerLeft() string { return "ausgetreten" }
func (textsDE) YouLeft() string {
	return "Du hast dieses Spiel verlassen. Du kannst es dir weiter ansehen."
}
func (textsDE) ErrCantLeave() string {
	return "Du kannst das Spiel jetzt nicht verlassen, z. B. nicht beim Ausbullen."
}
func (textsDE) EnterDart() string   { return "Dart eintragen" }
func (textsDE) HideKeypad() string  { return "Keypad ausblenden" }
func (textsDE) PlayOnBoard() string { return "Auf diesem Board spielen" }
func (textsDE) RemoveBoard() string { return "Kein Board: Darts von Hand eingeben" }
func (textsDE) RemoveBoardConfirm() string {
	return "Deine Spieler vom Board nehmen? Ab jetzt gibst du ihre Darts von Hand ein, zum Beispiel weil das Board kaputt ist."
}
func (textsDE) BoardInUseConfirm() string {
	return "Auf diesem Board läuft ein anderes Spiel. Das Board in dieses Spiel holen? Das andere Spiel läuft dann ohne Board weiter."
}
func (textsDE) AbortGameConfirm() string {
	return "Spiel wirklich abbrechen? Es zählt dann nicht in die Statistik."
}

func (textsDE) WinsGame(name string) string  { return "🏆 " + name + " gewinnt!" }
func (textsDE) Duration(clock string) string { return "Dauer " + clock }
func (textsDE) NewGame() string              { return "Neues Spiel" }
func (textsDE) GameAborted() string          { return "Spiel abgebrochen" }
func (textsDE) CheckoutSuggestion() string   { return "Vorschlag zum Auschecken" }
func (textsDE) PullDarts() string            { return "Pfeile ziehen" }
func (textsDE) ThenSet(n int) string         { return "Danach beginnt Set " + strconv.Itoa(n) + "." }
func (textsDE) ThenLeg(n int) string         { return "Danach beginnt Leg " + strconv.Itoa(n) + "." }
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
func (textsDE) ErrCompleted() string {
	return "Das Ergebnis steht fest, das Spiel kann nicht mehr geändert werden."
}
func (textsDE) Offline() string {
	return "Keine Verbindung zum Server – gleich noch einmal versuchen."
}
func (textsDE) ErrNotOver() string               { return "Das Spiel ist noch nicht vorbei." }
func (textsDE) NewVersion(version string) string { return "Version " + version + " verfügbar" }
func (textsDE) ErrGameOver() string              { return "Das Spiel ist schon vorbei." }
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
func (textsDE) GuestsHint() string              { return "Für Mitspieler ohne eigenen Account. Nur du siehst sie." }
func (textsDE) GuestName() string               { return "Name des Gastspielers" }
func (textsDE) Add() string                     { return "Hinzufügen" }
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
func (textsDE) TurnLimit() string   { return "Zeit pro Zug" }
func (textsDE) NoTurnLimit() string { return "Unbegrenzt" }
func (textsDE) QuickMatch() string  { return "Schnelles Spiel" }
func (textsDE) QuickMatchHint() string {
	return "Spielt gegen jemanden, der ungefähr so stark ist wie du: Du trittst einem wartenden Spiel bei oder wartest selbst auf einen Gegner. Je länger die Suche dauert, desto größer darf der Unterschied sein. X01 501 mit Double-Out oder Around the Clock, ein Leg, eine Minute pro Zug."
}
func (textsDE) FindOpponent() string { return "Gegner suchen" }
func (textsDE) QuickMatchWaiting() string {
	return "Suche einen Gegner, der ungefähr so stark ist wie du. Je länger du wartest, desto größer darf der Unterschied sein; lass die Seite offen, das Spiel startet von selbst."
}
func (textsDE) Strength() string  { return "Spielstärke" }
func (textsDE) AboutMine() string { return "Ungefähr wie meine" }
func (textsDE) TurnTime(seconds int) string {
	switch {
	case seconds == 60:
		return "1 Minute"
	case seconds%60 == 0:
		return strconv.Itoa(seconds/60) + " Minuten"
	}
	return strconv.Itoa(seconds) + " Sekunden"
}
func (textsDE) TurnLimitHint(skips int) string {
	return "Wer länger braucht, setzt aus; nach " + strconv.Itoa(skips) + " ausgesetzten Zügen in Folge ist man raus."
}
func (textsDE) TurnTimeLeft() string    { return "Verbleibende Zeit für den Zug" }
func (textsDE) Players() string         { return "Mitspieler" }
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
func (textsDE) CalibrationHint() string {
	return "Die Kalibrierung liegt auf dem Rechner mit den Kameras, nicht auf diesem Server: Das erledigt dort darts-o-cam."
}
func (textsDE) DownloadCam() string       { return "darts-o-cam herunterladen" }
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
func (textsDE) Email() string         { return "E-Mail" }
func (textsDE) EmailOptional() string { return "E-Mail (optional)" }
func (textsDE) EmailHint() string {
	return "Freiwillig. Es wird noch nichts daran verschickt; sie wird nur beim Account gespeichert. Leer lassen, um sie zu entfernen."
}
func (textsDE) SaveEmail() string { return "Speichern" }
func (textsDE) Incognito() string { return "Inkognito" }
func (textsDE) IncognitoHint() string {
	return "Deine Freunde sehen dich offline: weder, dass du online bist, noch dass du spielst. Ihren Status siehst du weiterhin."
}
func (textsDE) AppearOffline() string { return "Als offline anzeigen" }
func (textsDE) ErrInvalidEmail() string {
	return "Bitte eine gültige E-Mail-Adresse eingeben oder das Feld leer lassen."
}
func (textsDE) ErrEmailTaken() string {
	return "Diese E-Mail-Adresse wird schon von einem anderen Account verwendet."
}
func (textsDE) ErrTooManyLogins() string {
	return "Zu viele Fehlversuche. Bitte versuche es in ein paar Minuten nochmal."
}
func (textsDE) ErrCaptcha() string {
	return "Bitte die Prüfung bestehen, dass du kein Roboter bist, und es nochmal versuchen."
}
func (textsDE) DisplayName() string { return "Anzeigename" }
func (textsDE) DisplayNameHint() string {
	return "So heißt du im Spiel. Leer lassen, um den Benutzernamen zu verwenden."
}
func (textsDE) PasswordHint() string  { return "Mindestens 8 Zeichen." }
func (textsDE) CreateAccount() string { return "Account anlegen" }
func (textsDE) RegistrationClosed() string {
	return "Dieser Server nimmt gerade keine neuen Registrierungen an."
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
func (textsDE) PresenceOnline() string  { return "Online" }
func (textsDE) PresencePlaying() string { return "Im Spiel" }
func (textsDE) PresenceOffline() string { return "Offline" }

func (textsDE) StatsHint() string {
	return "Über alle beendeten Spiele und Turniere von dir und deinen Gastspielern."
}
func (textsDE) NoFinishedGames() string { return "Noch keine beendeten Spiele." }
func (textsDE) Heatmap() string         { return "Wo deine Darts landen" }
func (textsDE) AllModes() string        { return "Alle Modi" }
func (textsDE) RatePlayers() string     { return "Mitspieler bewerten" }
func (textsDE) RatePlayersHint() string {
	return "Andere sehen den Anteil der Daumen hoch ab 3 Bewertungen, zum Beispiel in der Liste der offenen Spiele."
}
func (textsDE) ThumbUp() string         { return "Guter Mitspieler" }
func (textsDE) ThumbDown() string       { return "Schlechter Mitspieler" }
func (textsDE) RatingsReceived() string { return "Daumen, die du von anderen Spielern bekommen hast" }
func (textsDE) LeftEarly() string       { return "Vorzeitig verlassen (Spiele / Turniere)" }
func (textsDE) BoardRequired() string   { return "Board nötig" }
func (textsDE) BoardOnlyOption() string { return "Nur Spieler mit Board dürfen mitspielen" }
func (textsDE) BoardOnlyHint() string {
	return "Alle brauchen ein Board, auch du und eingeladene Freunde: Die Kameras melden die Darts, niemand muss sie eintippen, und das Spiel läuft durch."
}
func (textsDE) NeedsBoard() string { return "Du brauchst ein Board, um beizutreten." }
func (textsDE) ErrBoardRequired() string {
	return "Dieses Spiel ist nur für Spieler mit Board. Wähle eins."
}
func (textsDE) ErrBoardBusy() string {
	return "Dieses Board ist in einem laufenden Spiel, das es braucht, und kann nicht weggenommen werden."
}
func (textsDE) ErrStarted() string { return "Es hat schon angefangen." }
func (textsDE) LobbyHint() string {
	return "Füge deine Spieler hinzu und lade Freunde ein, dann starte. Wer anfängt, wird beim Start ausgelost."
}
func (textsDE) FirstPlayerHint() string {
	return "Wer zuerst in der Lobby sitzt; weitere Spieler und Freunde kommen dort dazu."
}
func (textsDE) StartsWhenFull() string {
	return "Offen für Fremde: Es startet von selbst, sobald alle Plätze besetzt sind."
}
func (textsDE) StartTournament() string { return "Turnier starten" }
func (textsDE) AddGuest() string        { return "Gast hinzufügen" }
func (textsDE) Invite() string          { return "Einladen" }
func (textsDE) RemoveFromLobby() string { return "Entfernen" }
func (textsDE) Save() string            { return "Speichern" }
func (textsDE) Guests() string          { return "Gäste" }
func (textsDE) NoGuests() string {
	return "Noch keine Gäste. Du legst sie in der Lobby eines Spiels an."
}
func (textsDE) ErrPlayerBusy(name string) string {
	return name + " ist schon in einem anderen Spiel oder Turnier."
}
func (textsDE) HeatmapDarts(n int) string {
	return strconv.Itoa(n) + " von der Kamera erkannte Darts"
}
func (textsDE) HeatmapNone() string    { return "Darts, die die Kamera erkennt, erscheinen hier." }
func (textsDE) Count180() string       { return "180er" }
func (textsDE) GamesWinsLegs() string  { return "Spiele / Siege / Legs" }
func (textsDE) SetsWonLabel() string   { return "Sets gewonnen" }
func (textsDE) TournamentsWon() string { return "Turniere / gewonnen" }
func (textsDE) HighestFinish() string  { return "Höchstes Finish" }
func (textsDE) BestLeg() string        { return "Bestes Leg" }
func (textsDE) AvgLegTime() string     { return "Ø Leg-Dauer" }
func (textsDE) AvgGameTime() string    { return "Ø Spieldauer" }
func (textsDE) NDarts(n int) string    { return strconv.Itoa(n) + " Darts" }
func (textsDE) Games() string          { return "Spiele" }
func (textsDE) DartsHits() string      { return "Darts / Treffer" }

func (textsDE) InvitedToTournament() string {
	return "Du bist zu diesem Turnier eingeladen."
}
func (textsDE) WinsTournament(name string) string { return "🏆 " + name + " gewinnt das Turnier!" }
func (textsDE) LeaveTournament() string           { return "Turnier verlassen" }
func (textsDE) LeaveTournamentConfirm() string {
	return "Das Turnier wirklich verlassen? Es endet dann für alle, beendete Spiele bleiben erhalten."
}
func (textsDE) Bracket() string { return "Turnierbaum" }
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
func (textsDE) ErrGuestNameTaken() string {
	return "Du hast schon einen Spieler mit diesem Namen."
}
func (textsDE) ErrGameNotStarted() string {
	return "Das Spiel konnte nicht gestartet werden. Bitte die Einstellungen prüfen."
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
