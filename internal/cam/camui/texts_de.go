package camui

import (
	"strconv"
	"strings"
)

// textsDE are the German texts. Whatever isn't defined here comes from textsEN.
type textsDE struct{ textsEN }

func (textsDE) Number(f float64, digits int) string {
	return strings.Replace(strconv.FormatFloat(f, 'f', digits, 64), ".", ",", 1)
}

func (textsDE) NavCameras() string               { return "Kameras" }
func (textsDE) NavAddCamera() string             { return "Kamera hinzufügen" }
func (textsDE) NavSettings() string              { return "Einstellungen" }
func (textsDE) NewVersion(version string) string { return "Version " + version + " verfügbar" }
func (textsDE) Back() string                     { return "Zurück" }
func (textsDE) DetectionRunning(cameras int) string {
	if cameras == 1 {
		return "Erkennung läuft mit 1 Kamera."
	}
	return "Erkennung läuft mit " + strconv.Itoa(cameras) + " Kameras."
}
func (textsDE) DetectionWaiting() string           { return "Erkennung wartet auf eine kalibrierte Kamera." }
func (textsDE) DetectionStopped(err string) string { return "Erkennung angehalten: " + err }
func (textsDE) DetectionOff() string               { return "Erkennung ist ausgeschaltet." }

func (textsDE) TurnDetectionOff() string { return "Erkennung ausschalten" }
func (textsDE) TurnDetectionOn() string  { return "Erkennung einschalten" }
func (textsDE) ThrowsGoTo() string       { return "Würfe gehen an" }
func (textsDE) NotConnectedBefore() string {
	return "Mit keinem Server verbunden: erkannte Würfe erscheinen nur hier. Trage unter "
}
func (textsDE) NotConnectedAfter() string {
	return " den darts-o-mat-Server und den Kamera-Token des Boards ein."
}
func (textsDE) NoCameras() string {
	return "Noch keine Kameras. Füge unter „Kamera hinzufügen“ die Kameras deines Boards hinzu und kalibriere sie dann mit leerem Board."
}
func (textsDE) CalibrateHintBefore() string { return "Kalibriere jede Kamera einmal mit" }
func (textsDE) EmptyBoard() string          { return "leerem Board" }
func (textsDE) CalibrateHintAfter() string {
	return "; danach liegt auf ihrem Bild das Raster des Boards und die Erkennung nutzt sie sofort."
}
func (textsDE) RefreshImages() string      { return "Bilder aktualisieren" }
func (textsDE) Reconnect() string          { return "Kameras neu verbinden" }
func (textsDE) ReconnectHint() string      { return "Nach dem Ein- oder Ausstecken einer Kamera." }
func (textsDE) ImageOf(name string) string { return "Bild von " + name }
func (textsDE) PixelsPerMM() string        { return "Bildpunkte pro Millimeter auf dem Board" }
func (textsDE) Calibrated() string         { return "kalibriert" }
func (textsDE) NotCalibrated() string      { return "nicht kalibriert" }
func (textsDE) CalibratedForOtherSize(size string) string {
	return "Kalibriert für " + size + ", die Kamera liefert gerade eine andere Größe: bitte neu kalibrieren."
}
func (textsDE) Recalibrate() string      { return "Neu kalibrieren" }
func (textsDE) Calibrate() string        { return "Kalibrieren" }
func (textsDE) CheckGrid() string        { return "Raster prüfen" }
func (textsDE) RefineOnly() string       { return "Nur feinkalibrieren" }
func (textsDE) Remove() string           { return "Entfernen" }
func (textsDE) LatestDetections() string { return "Letzte Erkennungen" }
func (textsDE) ConfigHint(dir string) string {
	return "Kalibrierung und Einstellungen liegen auf diesem Rechner in " + dir + " und funktionieren mit jedem darts-o-mat-Server."
}
func (textsDE) ProblemNotConnected() string { return "Nicht angeschlossen." }
func (textsDE) ProblemMoved() string {
	return "Das Bild passt nicht mehr zum Bild bei der Kalibrierung (Kamera verschoben?). Bitte neu kalibrieren."
}

func (textsDE) NoEvents() string {
	return "Noch keine. Sie erscheinen hier, sobald die Erkennung Würfe sieht."
}
func (textsDE) DartsPulledOut() string { return "Pfeile herausgezogen" }
func (textsDE) NCameras(n int) string {
	return "(" + strconv.Itoa(n) + " " + plural(n, "Kamera", "Kameras") + ")"
}
func (textsDE) NotReported(err string) string     { return "nicht eingetragen: " + err }
func (textsDE) ReportedInGame(game string) string { return "eingetragen in Spiel " + game }
func (textsDE) Miss() string                      { return "Daneben" }
func (textsDE) DetectionTitle() string            { return "Erkennung" }
func (textsDE) UnlocatedNotice() string {
	return "Etwas hat sich verändert und es wurden keine Pfeile herausgezogen, aber keine Kamera hat den Pfeil auf dem Board gefunden: gezählt als Daneben, ohne Position."
}
func (textsDE) Camera() string           { return "Kamera" }
func (textsDE) DistanceToCenter() string { return "Abstand zur Mitte" }
func (textsDE) Resolution() string       { return "Auflösung (px/mm)" }
func (textsDE) Field() string            { return "Feld" }
func (textsDE) Hidden() string           { return "(verdeckt)" }
func (textsDE) Result() string           { return "✕ Ergebnis" }
func (textsDE) EventExplanation() string {
	return "Grenzen: Triple 99–107 mm, Doppel 162–170 mm, Bull bis 15,9 mm, Bullseye bis 6,35 mm. Auflösung: Pixel pro mm an dieser Stelle, zur Mitte hin / entlang des Rings. Das Ergebnis kombiniert die Abstände zur Mitte und die Winkel getrennt, jede Kamera zählt mit ihrer Auflösung in dieser Richtung: ab drei Kameras der gewichtete Median, bei zwei der gewichtete Mittelwert (nahe der Mitte pro Koordinate). Endet die Spitze einer Kamera an einem früheren Pfeil („verdeckt“), zählt sie nur, wenn keine andere Kamera den Pfeil sieht."
}
func (textsDE) Before() string { return "Vorher" }
func (textsDE) After() string  { return "Nachher" }

func (textsDE) RemoveTitle(name string) string    { return name + " entfernen" }
func (textsDE) RemoveQuestion(name string) string { return name + " entfernen?" }
func (textsDE) RemoveHint() string {
	return "Die Kamera wird vom Board entfernt, samt ihrer Kalibrierung. Du kannst sie später wieder hinzufügen und neu kalibrieren."
}
func (textsDE) Cancel() string { return "Abbrechen" }

func (textsDE) CalibrateTitle(name string) string   { return "Kalibrieren: " + name }
func (textsDE) CalibrateHeading(name string) string { return name + " kalibrieren" }
func (textsDE) ClickBefore() string                 { return "Klicke auf die Stelle, an der der Draht zwischen" }
func (textsDE) And() string                         { return "und" }
func (textsDE) ClickAfter() string                  { return "die Außenkante des Doppelrings kreuzt." }
func (textsDE) ClickHint() string {
	return "Ungefähr reicht, die Feinkalibrierung gleicht das aus. Ist die Stelle verdeckt oder nicht im Bild, überspringe sie."
}
func (textsDE) AllPointsDone() string { return "Alle Punkte durch." }
func (textsDE) PointsSet(points, min int) string {
	return strconv.Itoa(points) + " " + plural(points, "Punkt", "Punkte") + " gesetzt, mindestens " + strconv.Itoa(min) + " nötig."
}
func (textsDE) Skip() string              { return "Überspringen" }
func (textsDE) FinishCalibration() string { return "Fertig: feinkalibrieren und speichern" }
func (textsDE) StartOver() string         { return "Von vorn (neues Bild)" }
func (textsDE) ClickImage() string        { return "Kamerabild, hier klicken" }
func (textsDE) ErrTooFewPoints(min int) string {
	return "Mindestens " + strconv.Itoa(min) + " Punkte nötig."
}
func (textsDE) ErrPointsDontFit() string {
	return "Die Punkte passen nicht zusammen, bitte neu beginnen."
}
func (textsDE) ErrRefine(err error) string {
	return "Feinkalibrierung fehlgeschlagen (" + err.Error() + "); ist das Board leer und gut zu sehen?"
}
func (textsDE) AutoTap() string {
	return "Klicke einmal in den schmalen äußeren Ring (das Doppel) des Feldes 20."
}
func (textsDE) AutoTapHint() string {
	return "Das Board muss leer sein. Wo genau im Ring, ist egal; alles Weitere wird automatisch gefunden."
}
func (textsDE) ByHand() string { return "Stattdessen Drahtkreuzungen von Hand anklicken" }
func (textsDE) ErrAutoFailed() string {
	return "Das Board wurde mit diesem Klick nicht automatisch gefunden. Bitte klicke stattdessen die Drahtkreuzungen von Hand an, oder beginne von vorn und klicke das Doppel der 20 noch einmal."
}

func (textsDE) CheckTitle(name string) string   { return "Raster prüfen: " + name }
func (textsDE) CheckHeading(name string) string { return "Raster von " + name + " prüfen" }
func (textsDE) GridWrongTitle() string          { return "Das Raster passt nicht zum Board:" }
func (textsDE) GridWrong(found, searched int) string {
	return "nur " + strconv.Itoa(found) + " von " + strconv.Itoa(searched) + " Kanten und Drähten liegen dort, wo das Raster sie erwartet."
}
func (textsDE) GridWrongHint() string {
	return "Meist wurde beim Kalibrieren ein Draht verwechselt, z. B. 1/18 statt 20/1. Prüfe die Zahlen genau (sie stehen je nach Kamera auf dem Kopf) und kalibriere neu. Gegenprobe über die Farben: das Doppel der 20 ist rot, das der 1 grün."
}
func (textsDE) CheckExplanation() string {
	return "Auf dem aktuellen Bild gesucht: Kanten der farbigen Ringe und die Drähte zwischen den Zahlen. Blau ist das Raster, wie die Kalibrierung das Board sieht; die Punkte sind die gefundenen Kanten und Drähte. Liegen die blauen Linien auf den echten Drähten und sind fast alle Punkte grün, stimmt die Kalibrierung."
}
func (textsDE) FurtherOff() string             { return "weiter daneben" }
func (textsDE) Discarded() string              { return "verworfen (z. B. Pfeil davor)" }
func (textsDE) MeanDeviation(px string) string { return "mittlere Abweichung " + px + " px" }
func (textsDE) OnlyACheck() string {
	return "Das ist nur eine Prüfung, gespeichert wird nichts. Stecken Pfeile im Board, erscheinen an ihnen verworfene Punkte; das ist in Ordnung."
}
func (textsDE) FoundFeatures() string { return "Gefundene Kanten und Drähte" }

func (textsDE) DevicesHint() string {
	return "Alle Kameras an diesem Rechner. Füge die hinzu, die aufs Board schauen; danach wird sie kalibriert. Gleiche Kameras unterscheidet darts-o-cam an ihrem Bild vom Board, nicht am USB-Anschluss: Umstecken an andere Ports ist kein Problem."
}
func (textsDE) TakeNewImages() string { return "Neue Bilder aufnehmen" }
func (textsDE) WhichIsWhich() string {
	return "Welche ist welche? Halte eine Hand vor eine Kamera und nimm neue Bilder auf."
}
func (textsDE) NoDevices() string {
	return "Keine Kameras gefunden. Ist die Kamera angeschlossen und darf darts-o-cam darauf zugreifen?"
}
func (textsDE) ImageUnavailable(name string) string {
	return "Bild von " + name + " (nicht verfügbar)"
}
func (textsDE) OnBoardAs(name string) string { return "am Board als " + name }
func (textsDE) AddDevice() string            { return "Hinzufügen" }
func (textsDE) ErrAddCamera(err string) string {
	return "Kamera konnte nicht hinzugefügt werden: " + err
}

func (textsDE) Saved() string { return "Gespeichert und übernommen." }
func (textsDE) ServerHint() string {
	return "Leg auf dem Server unter „Boards“ dein Board an und erzeuge einen Kamera-Token. Erkannte Würfe gehen dann in das laufende Spiel auf diesem Board."
}
func (textsDE) CameraToken() string { return "Kamera-Token" }
func (textsDE) FrameSize() string   { return "Gewünschte Bildgröße" }
func (textsDE) FrameSizeHint() string {
	return "Jede Kamera nimmt ihren eigenen Modus, der dieser Größe am nächsten kommt; welche Größe sie liefert, steht auf ihrer Karte. Die Kameras werden dafür neu geöffnet. Eine Kalibrierung gilt nur für die Größe, in der sie gemacht wurde."
}
func (textsDE) FPS() string { return "Bilder pro Sekunde" }
func (textsDE) FPSHint() string {
	return "So viele Bilder pro Sekunde wertet die Erkennung aus; mehr reagiert schneller, braucht aber mehr Rechenleistung."
}
func (textsDE) Advanced() string { return "Erkennung (für Fortgeschrittene)" }
func (textsDE) AdvancedHint() string {
	return "Größen sind in Millimetern auf dem Board angegeben; darts-o-cam rechnet sie für jede Kamera nach ihrer Auflösung und ihrem Abstand in Bildpunkte um. Die Standardwerte stammen von einem Board mit drei Kameras bei 1280 × 720."
}
func (textsDE) Defaults() string                     { return "Standardwerte" }
func (textsDE) Save() string                         { return "Speichern" }
func (textsDE) ErrInvalidSettings(err string) string { return "Ungültige Einstellungen: " + err }
func (textsDE) LengthToWidth() string                { return "Länge : Breite" }
func (textsDE) Frames() string                       { return "Bilder" }
func (textsDE) DiffThreshold() string                { return "Farbunterschied" }
func (textsDE) DiffThresholdHint() string {
	return "Ab diesem Unterschied gilt ein Bildpunkt als verändert."
}
func (textsDE) WeakThreshold() string { return "Schwacher Farbunterschied" }
func (textsDE) WeakThresholdHint() string {
	return "Zählt nur neben stark veränderten Punkten; hält die dünne Spitze am Pfeil."
}
func (textsDE) Blur() string          { return "Weichzeichner" }
func (textsDE) BlurHint() string      { return "Radius gegen Bildrauschen." }
func (textsDE) MinArea() string       { return "Mindestgröße Pfeil" }
func (textsDE) MinAreaHint() string   { return "Kleinere Änderungen sind kein Pfeil." }
func (textsDE) MinSpeck() string      { return "Kleinster Fleck" }
func (textsDE) MinSpeckHint() string  { return "Kleinere Flecken werden als Rauschen ignoriert." }
func (textsDE) MinElongation() string { return "Mindest-Streckung" }
func (textsDE) MinElongationHint() string {
	return "So länglich muss der untere Teil des Pfeils (oder das ganze Stück) sein."
}
func (textsDE) Gap() string { return "Lücken überbrücken" }
func (textsDE) GapHint() string {
	return "Stücke mit Lücken bis zum Doppelten zählen zusammen (gestrichelter Barrel vor schwarzen Feldern)."
}
func (textsDE) AxisDistance() string { return "Abstand zur Achse" }
func (textsDE) AxisDistanceHint() string {
	return "Stücke so nah an der Pfeilachse gehören zum Pfeil (z. B. die Flight)."
}
func (textsDE) TipShift() string { return "Spitze zurücksetzen" }
func (textsDE) TipShiftHint() string {
	return "So weit wird die gefundene Spitze entlang des Pfeils zurückgesetzt: die Änderung im Bild reicht meist etwas über die echte Spitze hinaus (Schatten)."
}
func (textsDE) SampleStep() string { return "Bewegung: Abstand der Stichproben" }
func (textsDE) SampleStepHint() string {
	return "Für die Bewegungserkennung wird nur jeder so weit entfernte Bildpunkt verglichen."
}
func (textsDE) MotionThreshold() string { return "Bewegung: Farbunterschied" }
func (textsDE) MotionThresholdHint() string {
	return "Ab diesem Unterschied gilt eine Stichprobe als bewegt."
}
func (textsDE) MotionArea() string { return "Bewegung ab" }
func (textsDE) MotionAreaHint() string {
	return "So viel muss sich ändern, damit das Board als bewegt gilt."
}
func (textsDE) StableFrames() string { return "Ruhige Bilder" }
func (textsDE) StableFramesHint() string {
	return "So viele Bilder ohne Bewegung, bevor ausgewertet wird."
}
func (textsDE) TakeoutArea() string { return "Herausziehen ab" }
func (textsDE) TakeoutAreaHint() string {
	return "Ändert sich mehr, war es eine Hand bzw. herausgezogene Pfeile."
}

func (textsDE) TraceTitle() string { return "Erkennung im Detail" }
func (textsDE) TraceHint() string {
	return "Neu berechnet aus den gespeicherten Bildern, mit der aktuellen Kalibrierung und den aktuellen Einstellungen; die früheren Pfeile der Aufnahme werden mit erkannt. Ein Klick aufs Bild zeigt es in voller Größe."
}
func (textsDE) LegendSeed() string      { return "Barrel und Spitze" }
func (textsDE) LegendArea() string      { return "Rest des Pfeils (z. B. Flight)" }
func (textsDE) LegendPiece() string     { return "verändertes Stück, nicht der Pfeil" }
func (textsDE) LegendChanged() string   { return "veränderter Fleck" }
func (textsDE) LegendEarlier() string   { return "frühere Pfeile" }
func (textsDE) LegendFirstAxis() string { return "Achse der unteren Hälfte" }
func (textsDE) LegendAxis() string      { return "Achse nahe der Spitze" }
func (textsDE) LegendTipEnd() string    { return "Ende, aus dem die Spitze gemittelt wird" }
func (textsDE) LegendTip() string       { return "Spitze" }
func (textsDE) TraceChanged(pixels, pieces int) string {
	return strconv.Itoa(pixels) + " Bildpunkte verändert, " + strconv.Itoa(pieces) + " " + plural(pieces, "Stück", "Stücke")
}
func (textsDE) TraceDart(seed, area int, elongation string) string {
	return "Barrel und Spitze " + strconv.Itoa(seed) + " px (Länge : Breite " + elongation + "), ganzer Pfeil " + strconv.Itoa(area) + " px"
}
func (textsDE) TraceRetried() string {
	return "zunächst nicht wie ein Pfeil geformt: ohne die früheren Pfeile neu berechnet"
}
func (textsDE) TraceNoDart(err string) string { return "kein Pfeil: " + err }
func (textsDE) Zoom() string                  { return "Vergrößerung" }
func (textsDE) WholeImage() string            { return "Ganzes Bild" }
