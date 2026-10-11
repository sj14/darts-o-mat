# darts-o-mat

## Projektziel

Ziel des Projekts ist es, eine Alternative zu [Autodarts](https://autodarts.io) und [Scolia](https://scoliadarts.com) zu schaffen.

## Anforderungen

- **Mehrere Accounts mit Login** (umgesetzt):
  - Ein Account gehört fest zu genau einem Spieler (wird bei der Registrierung mit angelegt) und kann mehrere Boards haben.
  - Gastspieler haben keinen Account; sie gehören dem Account, der sie angelegt hat (`player.GuestOf`), und nur er kann sie in Spiele aufnehmen.
  - Freundesliste: Freundschaft ist gegenseitig (Anfrage, die andere Seite nimmt an; schicken sich beide eine Anfrage, gilt sie als angenommen). Entfernen beendet die Freundschaft, laufende gemeinsame Spiele bleiben.
  - Nur Freunde können zu Spielen eingeladen werden. Sie werden per Benutzername eingeladen und müssen zusagen. Solange eine Einladung offen ist, ist das Spiel in der Lobby und nimmt keine Darts an. Absagen oder Ausladen entfernt den Spieler aus dem Spiel; ein Spiel ohne Spieler wird gelöscht.
  - Darts eintragen/zurücknehmen dürfen der Gastgeber und zugesagte Accounts. Wer an einem Spiel nicht beteiligt ist, bekommt `ErrNotFound` (404), damit die Existenz nicht verraten wird.
  - Registrierung ist offen, abschaltbar mit `-registration=false`.

## Dokumentation

`README.md` richtet sich ausschließlich an Menschen: kurz, übersichtlich und ohne Vorwissen verständlich (was ist das, wie starte ich es, wie probiere ich es aus).
Keine Implementierungsdetails, Architektur oder Anweisungen für Agents in die README; die gehören in diese Datei.
Ändert sich etwas, das Nutzer betrifft (Start, Spielmodi, Bedienung), die README mit anpassen.

## Breaking Changes

Das Projekt ist noch nicht im Einsatz: Breaking Changes (Datenbankschema, API, Kalibrierungsformat) sind erlaubt, wenn sie den Code idiomatischer oder einfacher machen. Keine Migrationen und keine Kompatibilitäts-Umwege für alte Daten bauen.
Nach jeder Änderung, die eine bestehende Datenbank unbrauchbar macht, dem Nutzer am Ende ausdrücklich sagen, dass er die alte Datenbank (z. B. `darts-o-mat.db`) löschen muss. Entsprechend bei anderen Breaking Changes (API, Kalibrierungsdatei) sagen, was er anpassen oder neu erzeugen muss.

## Technologien

Neue Funktionen bauen auf dem bestehenden Stack auf; ohne Absprache keine zusätzlichen Frameworks oder Bibliotheken einführen.

- **Sprache:** Go (Version siehe `go.mod`)
- **HTTP-Routing:** [chi](https://github.com/go-chi/chi) v5
- **Frontend:** serverseitig gerenderte HTML-Templates mit [templ](https://templ.guide) (als Go-Tool in `go.mod` eingebunden; generieren mit `go tool templ generate`) und [htmx](https://htmx.org) für Interaktivität: Handler liefern templ-Fragmente, die htmx in die Seite tauscht, statt eigenem JavaScript bzw. einem JS-Framework
- **Design:** [Bootstrap 6 Alpha](https://getbootstrap.com) (nur CSS, ohne Bootstrap-JS). Die Klassennamen weichen von Bootstrap 5 ab, z. B. `btn-solid theme-primary` statt `btn-primary`, `alert theme-danger`, Breakpoints als `md:col-6`, Toggle-Buttons als `<label class="btn-check …"><input …></label>`, Checkboxen mit `class="check"`. Im Zweifel in `pkg/frontend/static/bootstrap.min.css` nachsehen statt v5-Wissen anzunehmen. Eigene Styles nur sparsam in `app.css`.
- **Statische Dateien:** htmx (samt SSE-Erweiterung) und Bootstrap liegen versioniert in `pkg/frontend/static` und werden per `go:embed` ausgeliefert, kein CDN (darts-o-mat soll auch ohne Internet laufen).
- **Datenbank:** PostgreSQL und SQLite werden beide unterstützt (Auswahl per `-db sqlite|postgres` und `-dsn`). Persistenz-Code muss mit beiden Datenbanken funktionieren.
- **ORM/Query-Builder:** [Bun](https://bun.uptrace.dev) (`pgdialect`/`pgdriver` für PostgreSQL, `sqlitedialect` mit dem cgo-freien Treiber `modernc.org/sqlite` für SQLite)
- **Kamera/Bildverarbeitung:** reines Go ohne cgo (`pkg/vision`); Kamerabilder kommen über **ffmpeg** als Rohbilder per Pipe (macOS: AVFoundation, Linux: V4L2), ffmpeg muss also installiert sein. GoCV war vorgesehen, für Intel-Macs mit macOS 26 gibt es aber keine OpenCV-Pakete (Homebrew müsste OpenCV mit 26 Abhängigkeiten stundenlang selbst bauen); deshalb am 2026-10-11 bewusst verworfen. Siehe [Kamera](#kamera). Geplant: ffmpeg durch ein eigenes Kamera-Paket ersetzen, siehe [Plan: Kamerazugriff ohne ffmpeg](#plan-kamerazugriff-ohne-ffmpeg).
- **Messaging (optional):** [NATS](https://nats.io) darf aufgenommen werden, wenn es gebraucht wird, z. B. um Würfe von der Kamera (`darts-o-cam`) an den Server (`darts-o-mat`) zu übertragen.

## Struktur

- `cmd/darts-o-mat`: Webserver
- `cmd/darts-o-cam`: Kamera-Werkzeug: `detect` (Würfe live erkennen, `-v`/`-save` zur Fehlersuche), `ui` (Kalibrier-Oberfläche), `calibrate` (Kalibrierung automatisch verfeinern), `overlay` (Kalibrierung prüfen), `snapshot` (ein Bild je Kamera), `analyze` (gespeicherte Würfe erneut auswerten), `score` (Bildpunkte aufs Board umrechnen). Meldet noch nichts an den Server.
- `pkg/api`: JSON-API unter `/api`
- `pkg/service`: Anwendungslogik inklusive Berechtigungsprüfung (Accounts, Gäste, Boards, Spiele, Einladungen, Darts), gemeinsam genutzt von API und Weboberfläche. Neue Logik gehört hierher, nicht in die Handler; Methoden bekommen den angemeldeten Account übergeben.
- `pkg/auth`: Session-Cookie (HttpOnly, SameSite=Lax) und Middleware, die den angemeldeten Account in den Request-Kontext legt. In der DB steht nur der SHA-256-Hash des Session-Tokens, Passwörter als bcrypt-Hash.
- `pkg/domain`: Fachlogik (Board, Spieler, Würfe, Spielmodi X01 und Around the Clock)
- `pkg/vision`: Kamera-Geometrie und Pfeilerkennung in reinem Go (Homographie, Kalibrierung, Bildvergleich, Watcher, ffmpeg-Kamerazugriff)
- `pkg/camui`: lokale Oberfläche von `darts-o-cam` (`ui`, läuft auch in `detect` mit): Kalibrieren, Raster prüfen, letzte Erkennungen
- `pkg/frontend`: Weboberfläche: Controller, templ-Views (`view`) und eingebettete statische Dateien (`static`)
- `pkg/repos/store`: Persistenz; ein gemeinsamer Bun-Store für beide Datenbanken, nur der Verbindungsaufbau (`NewPostgres`/`NewSQLite`) unterscheidet sich

## Kamera

`cmd/darts-o-cam` erkennt Würfe mit klassischer Bildverarbeitung in reinem Go (`pkg/vision`) und meldet sie mit `-server`/`-token` an darts-o-mat (sonst nur Ausgabe).

- **Kalibrierung bleibt beim Client** (Entscheidung des Nutzers, 2026-10-11): Die Kalibrierungsdatei liegt auf dem Rechner mit den Kameras, nie auf dem Server – dasselbe Board funktioniert so mit verschiedenen Servern; Server und Token kommen getrennt per `-server`/`-token`. Erstellt wird sie mit der lokalen Weboberfläche `darts-o-cam ui` (`pkg/camui`, Standard `localhost:8090`, templ + Bootstrap wie die Hauptanwendung): pro Kamera ein Standbild, der Nutzer klickt nacheinander Drahtkreuzungen am Doppelring an (`camui.Targets`, verdeckte überspringbar, mindestens 4; Klickposition per `<input type="image">`, Bild in Originalgröße, kein eigenes JavaScript), danach automatische Feinkalibrierung auf demselben Bild und Speichern samt Referenz- und Leer-Bild. Ohne Datei öffnet sie alle Kameras mit `-device-name`; unter „Kameras wählen“ (`/devices`, `vision.ListDeviceGroups`) wählt der Nutzer die Board-Kameras aus der ffmpeg-Geräteliste, gespeichert werden `device_name` und `device_model` (VID:PID aus `system_profiler`, nur zur Anzeige). Neu geöffnet werden die Kameras erst nach einem Neustart; startet die Oberfläche ohne passende Kameras, läuft sie trotzdem, damit man sie dort wählen kann. Grobe Klicks reichen: im Test 6 px daneben → 0,33 mm Abweichung zur sorgfältigen Kalibrierung. `detect` startet dieselbe Oberfläche mit (`-ui`, Standard `localhost:8090`, leer schaltet ab) auf denselben Kameras (`vision.Live` mit Bildzähler, damit `detect` kein Bild doppelt auswertet); eine Neukalibrierung geht sofort in die laufende Erkennung ein (`Watcher.SetToBoard`). Zusätzlich zeigt sie die letzten 20 Erkennungen (pro Kamera Bildausschnitt mit Spitze, Board-Skizze als SVG mit den Positionen der Kameras und dem Ergebnis) und „Raster prüfen“: Feinkalibrierung auf dem aktuellen Bild ohne Speichern, mit den gefundenen Ringkanten und Drähten eingefärbt nach Abweichung (`RefineResult.Features`).
- **Nicht umgesetzt (bewusst):** Aus Korrekturen im Spiel nachkalibrieren. Eine Korrektur nennt nur das Feld, nicht die Position, und die Fehler liegen nachweislich beim Finden der Spitze, nicht in der Kalibrierung (~0,3 mm). Offener Vorschlag an den Nutzer: nach jedem Herausziehen bei leerem Board automatisch feinkalibrieren (gleicht verrutschte Kameras aus) – erst nach seiner Entscheidung, weil das die Kalibrierungsdatei automatisch ändert.

- **Anbindung ans Spiel:** Pro Board ein Kamera-Token (Boards-Seite oder `POST /api/boards/{id}/token`), gespeichert nur als SHA-256-Hash in der Tabelle `tokens` (`board.Token`). `darts-o-cam` schickt `Authorization: Bearer <token>` an `POST /api/board/darts` bzw. `/api/board/takeout`. Der Dart geht an das neueste Spiel mit diesem Board, das bereit und nicht beendet ist (`service.AddBoardDart`), im Namen des Board-Besitzers – also gleiche Regeln und gleiches Undo wie beim Eintippen. Kein solches Spiel: `ErrNoActiveGame` (409).
- **Herausziehen** (`service.BoardTakeout`): ist die Aufnahme dann unvollständig, sind die fehlenden Darts nicht stecken geblieben (Abpraller, daneben) und werden als „Daneben“ eingetragen; danach wird das Herausziehen protokolliert und der Nächste ist dran.
- **Herausziehen erkennen:** Der Bildvergleich ist symmetrisch – ein verschwundener Pfeil sieht aus wie ein neuer. Deshalb vergleicht der Watcher die geänderte Stelle mit dem Bild vom **leeren Board** (`empty` in der Kalibrierung, legt `darts-o-cam calibrate` mit ab): sieht sie danach mehr nach leerem Board aus als vorher, wurde gezogen (`Watcher.removed`). Gemessen an 11 echten Ereignissen: Herausziehen ~0,05 (0,55 wenn ein Nachbarpfeil mitbewegt wurde), Würfe 4–17, Grenze 1. Jede Kamera stimmt einzeln ab, die Mehrheit entscheidet (Gleichstand: Summe): Eine Kamera, die beim Herausziehen ihre Belichtung nachregelt (cam2 am Entwicklungs-Board, Hand vor der Linse), ändert sich fast im ganzen Bild und überstimmte in der Summe die anderen (echtes Herausziehen 0,06/1,9/0,08 → Summe 1,19, als Wurf gewertet; `TestWatcherRemovedExposure`, `testdata/exposure`). Die Abstimmung wertete auch einen Doppelwurf (leer → 2 Pfeile in einem Ereignis) richtig, den die Summe als Herausziehen sah. Funktioniert auch für Pfeile, die schon vor dem Start steckten (ohne das zählte das Herausziehen alter Pfeile als Wurf, im echten Spiel passiert). Ohne Leer-Bilder Rückfall: Änderung überwiegend dort, wo seit dem letzten Herausziehen erkannte Pfeile stecken (dann mit leerem Board starten).
- **Live-Anzeige per Server-Sent Events:** Der Service meldet jede Änderung an einem Spiel (`Service.Subscribe`/`notify`: Dart, Undo, Einladung beantwortet – egal ob über Weboberfläche, API oder Kamera). Die Spielseite verbindet sich mit `GET /games/{id}/events` (htmx-SSE-Erweiterung, vendored in `static`); der Server schickt bei jeder Meldung das für den jeweiligen Betrachter gerenderte Spielfeld als Ereignis `board`, das Element `#board` tauscht sich damit aus (`sse-swap`). Mehrere Änderungen während eines laufenden Renderns werden zu einer Meldung zusammengefasst. Keepalive-Kommentar alle 25 s. Läuft nur innerhalb eines Server-Prozesses; für mehrere Instanzen bräuchte es einen Bus (z. B. NATS).


- **Board-Geometrie:** `pkg/domain/dartboard` (Normmaße in mm, Ursprung im Bull, Y zeigt zur 20; `Score` wertet eine Position).
- **Kalibrierung** in zwei Schritten:
  1. Grob von Hand: pro Kamera mindestens 4 Bildpunkte, an denen die Außenkante des Doppelrings einen Draht kreuzt (`between: [a, b]` im Uhrzeigersinn); daraus eine Homographie Bild → Board. Zum Finden der Punkte: Zuschnitte mit Raster per ffmpeg, Gegenprobe über die Ringfarben (Doppel/Triple der 20 rot, der 1 grün, abwechselnd). Es reicht grob: die Feinkalibrierung kommt von 15–34 mm daneben noch auf 0,3 mm an dasselbe Ergebnis (`TestRefineCalibrationReal`).
  2. Automatisch fein (`darts-o-cam calibrate`, `vision.RefineCalibration`) mit einem Bild vom **leeren** Board pro Kamera: rund 600 Merkmale – Kanten der farbigen Ringe als Sprung in der Farbsättigung (Ringe rot/grün, Felder schwarz/weiß), Drähte als Wechsel rot↔grün entlang Triple- und Doppelring. Angepasst wird nur der Abstand quer zur jeweiligen Kante (Punkt-zu-Linie, Gauss-Newton); eine Kante als vollen Punkt zu behandeln zog das Ergebnis zur Startkalibrierung zurück. Suchfenster pro Ring auf < halben Abstand zur Nachbarkante begrenzt (sonst springt die Suche am kleinen Bull auf die falsche Kante). Ergebnis steht als `refined` in der Kalibrierung und hat Vorrang vor den Punkten. Restabweichung real ~1,1–1,4 px (synthetisch 0,3 px): Kantenrauschen auf Sisal/Drähten, **nicht** Linsenverzeichnung (ein radialer Verzeichnungsterm senkte sie nur um 6–8 %, daher nicht eingebaut).
  `darts-o-cam overlay` zeichnet das berechnete Raster ins Kamerabild; erst wenn es sichtbar passt, gilt die Kalibrierung. Kalibrierung des Entwicklungs-Boards: `pkg/vision/testdata/calibration.json` (Referenzbilder und Bilder vom leeren Board daneben). Pixelmitten liegen in `vision` auf ganzen Zahlen.
- **Erkennung** (`vision.DetectDart`): Farbdifferenz zum Bild vor dem Wurf (Graustufen reichen nicht: silberner Pfeil auf rotem/grünem Bull ist gleich hell), 5×5 weichgezeichnet, Hysterese-Schwellwerte 25/12, Flecken < 15 px verworfen. Die Änderung zerfällt bei echten Pfeilen in Stücke (dunkler Schaft vor schwarzem Feld trennt Flight und Barrel; Barrel vor schwarzen Feldern nur gestrichelt), deshalb: Stücke mit Lücken bis 6 px zusammenfassen; Ausgangspunkt ist das größte Stück, dessen **untere Hälfte länglich** ist (Barrel und Spitze; die aus der Nähe große Flight würde Achse und Längen-Breiten-Verhältnis verzerren); dazu kommen Stücke auf dessen Achse (Flight, abgebrochene Spitze), Flecken daneben nicht. Spitze ist das **untere** Ende entlang dieser Achse (die Kameras schauen vom Rand flach über das Board, Pfeile stehen nach oben). Synthetische Pfeile: Spitze auf ≤ 5 px genau (das Weichzeichnen verwäscht die letzten Pixel der dünnen Spitze). Eine Verlängerung der Spitze entlang der Achse im nicht weichgezeichneten Bild half bei synthetischen Pfeilen, lief bei echten aber in den Schatten der Spitze und verschlechterte die Ergebnisse (gemessen an 5 echten Würfen) – nicht wieder einbauen ohne neue Messung. Sieht eine Kamera nur die Flight (Rest verdeckt oder schwarz auf schwarz), meldet sie keine Spitze, statt zu raten.
- **Fusion:** Spitze jeder Kamera aufs Board umrechnen, Ergebnisse mehr als 225 mm vom Zentrum verwerfen (`MaxBoardRadius`, z. B. ein Reflex in der Bildecke), Median pro Koordinate. Eine Kamera, die die Einstichstelle nicht sieht (Pfeil zeigt von ihr weg, gestauchter Bildrand), lag im Test 15 mm daneben; der Median fängt das ab.
- **Ablauf** (`vision.Watcher`): Bewegung über jedes 4. Pixel mit Schwellwert 40 (Kamerarauschen ≤ 5 Stichproben, ein Pfeil ~160), warten bis 3 Bilder ruhig sind, dann auswerten. Sehr große Änderungen oder keine erkennbare Spitze gelten als Herausziehen (`takeout`).
- **Stand der Genauigkeit:** Mit 8 echten Referenzpfeilen (zwei Runden, Spitzen von Hand verortet, Felder vom Spieler bestätigt) stimmen 7; der achte liegt 1 mm neben dem Draht auf der falschen Seite (`TestRealDarts`). Einzelne Kameras liegen öfter daneben (ferner, gestauchter Bildrand; schwarzer Pfeil vor schwarzem Segment unsichtbar), der Median fängt das ab. Echte Würfe mit automatischer Erkennung sind noch nicht getestet. Erste echte Würfe (4, mit `-save` aufgezeichnet): alle Felder richtig, 11 von 12 Kamerabildern liefern eine Spitze, die Kameras liegen pro Wurf 2–16 mm auseinander; zwei davon sind Regressionstests (`TestRealThrows`, `testdata/throws`). Zweite Runde (von Hand gesteckt, beim Herausziehen aufgezeichnet – Vorher/Nachher vertauscht ist für den Vergleich egal): D19 und äußeres Bull richtig; T20 als S20, weil der Pfeil am inneren Triple-Draht steckt und alle drei Kameras übereinstimmend 1–2 mm innen messen; Bullseye nicht erkannt, weil beim Herausziehen der Nachbarpfeil mitbewegt wurde (zwei Änderungen gleichzeitig). Die verbleibenden Fehler liegen bei Pfeilen direkt am Draht: nächster Hebel ist eine genauere Kalibrierung (z. B. automatisch über die Farbwechsel der Ringe). Mit Feinkalibrierung: Pfeile, die nicht direkt am Draht stecken, werden zuverlässig richtig gewertet; Pfeile < 1,5 mm am Draht fallen mal so, mal so aus (Feinkalibrierung behebt S5/S12 und bringt T20 auf 0,2 mm an die Grenze, dafür kippt S3/S17) – dort ist eher die Spitzenerkennung als die Kalibrierung der Engpass. `TestRealDarts` prüft solche Pfeile deshalb nicht, sondern protokolliert sie. Weiterer Fall (2026-10-11): T4 am inneren Triple-Draht als S4 (Kameras 99,2/99,0/96,2 mm, Median 97,6). Ausprobiert und verworfen: Spitze entlang der Achse ans Ende schieben (statt Mittelpunkt der letzten 3 px), schwächerer Weichzeichner (±0,5 mm), Schnitt der Pfeilachsen-Linien statt Median (T4 nur bis 98,3 mm, eine Fehlmessung zog das Bullseye 90 mm weg).
- **Kameras finden:** Die AVFoundation-Reihenfolge unter macOS ändert sich ständig (beobachtet: FaceTime-Kamera mal Index 0, mal 2, mal 3), und alle Board-Kameras heißen gleich. Deshalb stehen in der Kalibrierung keine festen Indizes, sondern `device_name` und pro Kamera ein Referenzbild (`reference`). `vision.OpenCameras` öffnet alle Geräte mit dem Namen und ordnet sie per Bildvergleich zu (`AssignCameras`, 64×36-Graustufen-Miniatur; gemessen: gleiche Kamera 6–8, andere Board-Kamera 47–61, FaceTime 67–69, Grenze 25). Eine feste `device` (z. B. `/dev/video0` unter Linux) geht weiterhin. **Stabiler Bezeichner:** Die drei Autodarts-Kameras haben dieselbe VID:PID (1e45:8022) *und* dieselbe Seriennummer (200910120001); macOS-Unique-IDs und Linux-`/dev/v4l/by-path` hängen am USB-Port. Es gibt also keine Hardware-ID pro Kamera – Name (plus Modell) wählt die Gruppe, das Bild die einzelne Kamera. Das überlebt Umstecken an andere Ports.
- **Fehlersuche:** `darts-o-cam detect -v` meldet jede Bewegung, `-save <dir>` speichert bei jedem Ereignis die Bilder aller Kameras vor/nach. Achtung: Bilder können Personen zeigen (z. B. falsch zugeordnete Laptop-Kamera), nicht ins Repo legen.
- **Bekannte Grenzen:** keine Behandlung von Abprallern, verdeckten Spitzen durch bereits steckende Pfeile, Pfeilen die beim Herausziehen hängen bleiben.

### Hardware (Entwicklungsrechner, Stand 2026-10-11)

Board mit drei Kameras („Autodarts DIY Cam“), angeschlossen über einen USB-C-Adapter an ein MacBook Pro (Intel, macOS 26):

- Die drei Kameras hängen an einem USB-2.0-Hub auf dem Board (`0x214b:0x7250`), alle mit `0x1e45:0x8022` und **derselben Seriennummer** `200910120001`. Unterscheidbar sind sie nur über den USB-Port (macOS Location ID `0x14111200`, `0x14111300`, `0x14111400`; Kamera-Unique-ID = Location ID + Vendor + Product, z. B. `0x141112001e458022`). Die Zuordnung gilt nur, solange Board und Adapter gleich gesteckt sind.
- UVC, unter macOS per AVFoundation (ffmpeg-Index 1–3; Index 0 ist die eingebaute FaceTime-Kamera). Modi bis 1280×720 bei 30 fps; alle drei liefern gleichzeitig volle 30 fps in 720p (komprimiert über USB 2.0).
- Die Kameras sitzen rundum am Lichtring und schauen schräg von verschiedenen Seiten auf das Board. Die ffmpeg/AVFoundation-Indizes sind nicht stabil, siehe „Kameras finden“ oben.
- Im selben Adapter stecken außerdem ein VIA-Hub, Kartenleser und Realtek-LAN; die gehören nicht zum Board.

Weitere Optionen, falls der jetzige Ansatz nicht reicht (Pflegestand der Bibliotheken vor der Entscheidung prüfen):

- **GoCV/OpenCV:** wenn auf der Zielplattform ein OpenCV-Paket verfügbar ist (z. B. Linux/Raspberry Pi), für robustere Bausteine (Konturen, Linienanpassung, Kamerakalibrierung gegen Linsenverzeichnung).
- **Neuronales Netz über ONNX Runtime** (z. B. [onnxruntime_go](https://github.com/yalue/onnxruntime_go), ebenfalls cgo): ein trainiertes Modell erkennt die Dartspitzen direkt, robuster bei Licht und Verdeckung, braucht aber Trainingsdaten und ein Modell.
  Erst angehen, wenn der klassische Ansatz nicht zuverlässig genug ist.
- **Python mit OpenCV als eigener Dienst**, der Würfe per NATS/HTTP meldet: nur als letzter Ausweg, weil es den Go-Stack verlässt.

### Plan: Kamerazugriff ohne ffmpeg

Entschieden mit dem Nutzer am 2026-10-11, **noch nicht umgesetzt**:

- **Ziel:** darts-o-cam ohne externe Programme auf Linux, macOS und möglichst Windows. Ohne cgo ist erwünscht, aber kein Muss.
- **Eigenes Paket** (z. B. `pkg/camera`) mit einem Backend je Betriebssystem und kleiner Schnittstelle: Geräte auflisten (Name, ID, Modell), Kamera mit Breite/Höhe/fps öffnen, Bilder als Stream (neues Bild wird signalisiert, kein „letztes Bild abholen“). `pkg/vision` hängt nur noch an dieser Schnittstelle; Auswahl per Name und Zuordnung per Bild (siehe „Kameras finden“) bleiben.
- **Reihenfolge:**
  1. macOS: AVFoundation über [purego](https://github.com/ebitengine/purego) (Apache 2.0, vom Nutzer freigegeben) statt cgo, damit kein Xcode zum Bauen nötig ist und Cross-Kompilieren geht. Zur Laufzeit registrierte Delegate-Klasse, Callbacks auf der Dispatch-Queue, Kamera-Berechtigung beachten. `AVCaptureDevice` liefert `localizedName`, `uniqueID` und `modelID` (bei UVC mit VendorID/ProductID) – damit entfällt auch `system_profiler`. Außerdem Belichtung und Weißabgleich nach dem Einschwingen sperren (`exposureMode`/`whiteBalanceMode` locked, falls die UVC-Kameras es unterstützen): Die automatische Belichtung ändert sonst beim Herausziehen das ganze Bild.
  2. Linux: V4L2 per ioctl mit `golang.org/x/sys/unix`, MJPEG mit `image/jpeg` dekodieren (bei 3 Kameras an USB 2.0 nötig); Geräteliste aus `/sys/class/video4linux`.
  3. Windows: Media Foundation per `syscall` (COM über vtables), am aufwändigsten.
- **Kein ffmpeg-Rückfall** (Entscheidung des Nutzers): Mit dem Umbau fliegt ffmpeg ganz raus; Linux und Windows gehen dann erst wieder, wenn ihr Backend fertig ist.
- **Geprüfte Bibliotheken:**
  - [gen2brain/cam2ip](https://github.com/gen2brain/cam2ip) (Paket `camera`): ohne cgo auf allen drei Systemen. Am Entwicklungs-Board getestet: 3 Kameras gleichzeitig mit 1280×720 bei ~29,5 fps. Verworfen wegen GPL-3.0; außerdem gibt `Read()` nur das letzte Bild im selben Puffer zurück, ohne auf ein neues zu warten, und liefert kein Modell. Den Code nicht übernehmen (GPL), das eigene Backend nach Apple-Doku schreiben.
  - [svanichkin/gocam](https://github.com/svanichkin/gocam) (MIT): unbrauchbar. Es öffnet nur die Standardkamera bzw. `/dev/video0` mit globalem Zustand (keine drei Kameras), unter macOS fest 352×288, keine Geräteliste, cgo.
  - Reine Linux-Pakete (go4vl, ch3ri0ur/go-v4l2, korandiz/v4l) decken nur Linux ab.

## Spielstand

Gespeichert werden nur das Spiel (Modus, Einstellungen, Spieler in Wurfreihenfolge) und die geworfenen Darts.
Der aktuelle Stand wird immer durch Abspielen der Darts berechnet (`game.Game.Play`, je Modus eine `Play`-Funktion).
Neue Spielmodi folgen demselben Muster; Spielstand wird nicht zusätzlich in der Datenbank abgelegt.

Spielende (alle Modi, `pkg/domain/game/turn`): Nach dem ersten Checkout dürfen die übrigen Spieler der Runde nachziehen.
Das Spiel endet mit dieser Runde; alle, die darin auschecken, gewinnen gemeinsam (`winner_ids`).
X01 kennt die Out-Modi `single`, `double` und `master` (Doppel oder Triple).
Spiele auf einem Board (`game.Game.WaitsForTakeout`) warten nach jeder Aufnahme (auch nach Bust) aufs Herausziehen der Pfeile, bevor der Nächste dran ist; der letzte Wurf, der das Spiel beendet, wartet nicht. Das Herausziehen ist ein Eintrag im selben Protokoll wie die Darts (`throw.Dart.Takeout`, Tabelle `darts`, Reihenfolge = ID) und wird beim Nachrechnen berücksichtigt (`turn.Log.Replay`). Rückgängig löscht einfach den letzten Eintrag. Die Kamera meldet es (`POST /api/board/takeout`), sonst bestätigen die Spieler es auf der Spielseite („Pfeile gezogen“, `POST /games/{id}/takeout` bzw. `/api/games/{id}/takeout`). Ein Dart, während gewartet wird, ist beim Nachrechnen ein Fehler (`turn.ErrWaitingForTakeout`), neue Darts werden deshalb abgelehnt. `turn.End`/`Finish` zählen den Dart mit, der die Aufnahme beendet (Bust, Checkout), damit die wartende Aufnahme die richtige Dartzahl zeigt. Spiele ohne Board wechseln wie bisher nach drei Darts.
Darts lassen sich nachträglich korrigieren (`service.EditDart`; in der Oberfläche jeden Dart im Verlauf bzw. der laufenden Aufnahme anklicken): Der Eintrag wird geändert und alles neu nachgerechnet. Abgelehnt wird, wenn das Protokoll danach nicht mehr aufgeht (Replay-Fehler, z. B. Spiel wäre vor späteren Darts vorbei) oder wenn sich ändern würde, wer spätere Darts geworfen hat (`ErrChangesThrowers`, z. B. Überworfen mitten in der Aufnahme) – dann stattdessen zurücknehmen. Dafür liefern die Modi pro Protokolleintrag den Werfer (`State.Throwers`, `game.Throwers`).
Abbrechen (`service.AbortGame`, nur der Gastgeber, auch in der Lobby): setzt `games.aborted_at`; das Spiel bleibt samt Darts erhalten, nimmt aber nichts mehr an (`ErrAborted`, 409), `CanThrow` ist dann false, offene Einladungen verschwinden aus `Invitations`, die Kamera überspringt es (`activeGame`) und die Statistik zählt es nicht (es ist nie `GameOver`). Beendete Spiele lassen sich nicht abbrechen.
Der X01-Stand enthält zusätzlich den Verlauf aller Aufnahmen (`x01.State.Turns`: Darts, Punkte, Rest, überworfen, Checkout; die letzte kann noch laufen); die Spielseite zeigt ihn neueste zuerst.

## Statistik

Statistiken werden wie der Spielstand beim Abspielen der Darts berechnet und nicht gespeichert:

- Pro Spiel hängt jeder Modus sie an seinen Spielerstand (`x01.PlayerState.Stats`, `aroundtheclock.PlayerState.Stats`), sie stehen also in jedem `state` der API.
- X01: 3-Dart-Average = erzielte Punkte × 3 / geworfene Darts (Bust-Aufnahmen zählen 0 Punkte, ihre Darts zählen mit); Aufnahmen zählen in Höchste Aufnahme und 100+/140+/180 erst, wenn sie vorbei sind.
- `pkg/domain/stats` summiert über alle beendeten Spiele (laufende zählen nicht) für den eigenen Spieler und die Gäste eines Accounts; Siege schließen Unentschieden ein.
- Die Gesamtstatistik spielt dafür alle Spiele des Accounts neu ab. Wird das bei sehr vielen Spielen zu langsam, ist ein Cache pro beendetem Spiel der naheliegende nächste Schritt.

## API

Alle Endpunkte unter `/api` außer `register`/`login`/`logout` brauchen das Session-Cookie aus `POST /api/login`.

- `POST /api/register` (`{"username", "name", "password"}`), `POST /api/login` (`{"username", "password"}`), `POST /api/logout`
- `GET /api/players` (eigener Spieler + Gäste), `POST /api/players` (`{"name"}`, legt einen Gast an)
- `GET /api/boards`, `POST /api/boards` (`{"name"}`), `DELETE /api/boards/{id}`
- `GET /api/games`, `POST /api/games`, z. B. `{"mode": "x01", "player_ids": [1, 2], "invite": ["ben"], "board_id": 1, "x01": {"start_score": 501, "out_mode": "double"}}`
  oder `{"mode": "around_the_clock", "player_ids": [1], "around_the_clock": {"bull": true, "multiplier_skips": false}}`; `player_ids` nur eigener Spieler und eigene Gäste; `invite` nur Freunde, sie kommen ans Ende der Wurfreihenfolge
- `GET /api/games/{id}`: Spiel, Spieler, Einladungen, Protokoll (`darts`, inklusive Einträgen mit `takeout: true`), berechneter Stand sowie `ready`, `is_owner`, `can_throw`, `invited`
- `POST /api/games/{id}/darts`: `{"number": 20, "multiplier": 3}` oder `{"miss": true}`; Bull ist `25`
- `POST /api/games/{id}/takeout`: Pfeile wurden gezogen (nur bei Spielen auf einem Board von Bedeutung)
- `POST /api/games/{id}/abort`: Spiel abbrechen (nur Gastgeber), danach steht `aborted_at` im Spiel
- `DELETE /api/games/{id}/darts/last`: letzten Dart rückgängig machen
- `PUT /api/games/{id}/darts/{dartID}`: Dart korrigieren (Body wie beim Eintragen)
- `POST /api/boards/{id}/token`: neuer Kamera-Token für ein eigenes Board (ersetzt den alten), `GET /api/boards` zeigt `has_token`
- Für Kameras, mit `Authorization: Bearer <Board-Token>` statt Session: `POST /api/board/darts` (Dart wie oben, geht ans laufende Spiel des Boards), `POST /api/board/takeout`
- `GET /api/stats`: Gesamtstatistik des eigenen Spielers und der Gäste
- `GET /api/friends` (Freunde, eingehende und gesendete Anfragen), `POST /api/friends` (`{"username"}`, nimmt eine bereits eingegangene Anfrage an), `POST /api/friends/{accountID}/accept`, `DELETE /api/friends/{accountID}` (entfernen, ablehnen oder zurückziehen)
- `GET /api/invitations`, `POST /api/games/{id}/accept`, `POST /api/games/{id}/decline`, `DELETE /api/games/{id}/invitations/{playerID}` (Gastgeber lädt aus)

## Testen

- Regeln und Service (Accounts, Rechte, Einladungen gegen In-Memory-SQLite): `go test ./pkg/...`
- Weboberfläche: mit [Lightpanda](https://github.com/lightpanda-io/browser) statt Headless Chrome, z. B. `lightpanda fetch <url> --inject-script-file klicks.js --wait-script '…' --dump semantic_tree_text --dump-selector '#board'`.
  Lightpanda führt JavaScript/htmx aus, rendert aber keine Pixel; für echte Screenshots gibt es damit keinen Ersatz.
