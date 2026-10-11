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

## Technologien

Neue Funktionen bauen auf dem bestehenden Stack auf; ohne Absprache keine zusätzlichen Frameworks oder Bibliotheken einführen.

- **Sprache:** Go (Version siehe `go.mod`)
- **HTTP-Routing:** [chi](https://github.com/go-chi/chi) v5
- **Frontend:** serverseitig gerenderte HTML-Templates mit [templ](https://templ.guide) (als Go-Tool in `go.mod` eingebunden; generieren mit `go tool templ generate`) und [htmx](https://htmx.org) für Interaktivität: Handler liefern templ-Fragmente, die htmx in die Seite tauscht, statt eigenem JavaScript bzw. einem JS-Framework
- **Design:** [Bootstrap 6 Alpha](https://getbootstrap.com) (nur CSS, ohne Bootstrap-JS). Die Klassennamen weichen von Bootstrap 5 ab, z. B. `btn-solid theme-primary` statt `btn-primary`, `alert theme-danger`, Breakpoints als `md:col-6`, Toggle-Buttons als `<label class="btn-check …"><input …></label>`, Checkboxen mit `class="check"`. Im Zweifel in `pkg/frontend/static/bootstrap.min.css` nachsehen statt v5-Wissen anzunehmen. Eigene Styles nur sparsam in `app.css`.
- **Statische Dateien:** htmx und Bootstrap liegen versioniert in `pkg/frontend/static` und werden per `go:embed` ausgeliefert, kein CDN (darts-o-mat soll auch ohne Internet laufen).
- **Datenbank:** PostgreSQL und SQLite werden beide unterstützt (Auswahl per `-db sqlite|postgres` und `-dsn`). Persistenz-Code muss mit beiden Datenbanken funktionieren.
- **ORM/Query-Builder:** [Bun](https://bun.uptrace.dev) (`pgdialect`/`pgdriver` für PostgreSQL, `sqlitedialect` mit dem cgo-freien Treiber `modernc.org/sqlite` für SQLite)
- **Kamera/Bildverarbeitung:** [GoCV](https://github.com/hybridgroup/gocv) (OpenCV-Anbindung für Go) verwenden, wo es sinnvoll ist. GoCV braucht cgo und ein installiertes OpenCV; das betrifft nur `darts-o-cam`. Der Server (`darts-o-mat`) bleibt cgo-frei und darf GoCV nicht importieren. Alternativen siehe [Kamera](#kamera).
- **Messaging (optional):** [NATS](https://nats.io) darf aufgenommen werden, wenn es gebraucht wird, z. B. um Würfe von der Kamera (`darts-o-cam`) an den Server (`darts-o-mat`) zu übertragen.

## Struktur

- `cmd/darts-o-mat`: Webserver
- `cmd/darts-o-cam`: Kamera-Client, der Würfe erkennt und an den Server meldet
- `pkg/api`: JSON-API unter `/api`
- `pkg/service`: Anwendungslogik inklusive Berechtigungsprüfung (Accounts, Gäste, Boards, Spiele, Einladungen, Darts), gemeinsam genutzt von API und Weboberfläche. Neue Logik gehört hierher, nicht in die Handler; Methoden bekommen den angemeldeten Account übergeben.
- `pkg/auth`: Session-Cookie (HttpOnly, SameSite=Lax) und Middleware, die den angemeldeten Account in den Request-Kontext legt. In der DB steht nur der SHA-256-Hash des Session-Tokens, Passwörter als bcrypt-Hash.
- `pkg/domain`: Fachlogik (Board, Spieler, Würfe, Spielmodi X01 und Around the Clock)
- `pkg/frontend`: Weboberfläche: Controller, templ-Views (`view`) und eingebettete statische Dateien (`static`)
- `pkg/repos/store`: Persistenz; ein gemeinsamer Bun-Store für beide Datenbanken, nur der Verbindungsaufbau (`NewPostgres`/`NewSQLite`) unterscheidet sich

## Kamera

Noch nicht umgesetzt. Vorgesehener Ansatz für den Start ist klassische Bildverarbeitung: Bild vor und nach dem Wurf vergleichen, die Dart-Linie finden und per Homographie auf das Board umrechnen. Dafür spart GoCV am meisten Arbeit.

Alternativen, falls sich GoCV nicht bewährt (Pflegestand der Bibliotheken vor der Entscheidung prüfen):

- **Reines Go ohne cgo:** [go4vl](https://github.com/vladimirvivien/go4vl) für die Kamera (nur Linux/V4L2, z. B. Raspberry Pi), [bild](https://github.com/anthonynsimon/bild) oder [imaging](https://github.com/disintegration/imaging) für Bildoperationen, [gonum](https://gonum.org) für Homographie und Geradenanpassung.
  Vorteil: kein OpenCV, einfaches Cross-Compiling, ein Binary. Nachteil: Konturen, Linienanpassung und Kalibrierung selbst schreiben; Kamerazugriff auf macOS bräuchte wieder cgo.
  Sinnvoll, wenn einfaches Ausliefern (z. B. auf einen Raspberry Pi) wichtiger ist als schnelles Vorankommen.
- **Neuronales Netz über ONNX Runtime** (z. B. [onnxruntime_go](https://github.com/yalue/onnxruntime_go), ebenfalls cgo): ein trainiertes Modell erkennt die Dartspitzen direkt, robuster bei Licht und Verdeckung, braucht aber Trainingsdaten und ein Modell.
  Erst angehen, wenn der klassische Ansatz nicht zuverlässig genug ist.
- **Python mit OpenCV als eigener Dienst**, der Würfe per NATS/HTTP meldet: nur als letzter Ausweg, weil es den Go-Stack verlässt.

## Spielstand

Gespeichert werden nur das Spiel (Modus, Einstellungen, Spieler in Wurfreihenfolge) und die geworfenen Darts.
Der aktuelle Stand wird immer durch Abspielen der Darts berechnet (`game.Game.Play`, je Modus eine `Play`-Funktion).
Neue Spielmodi folgen demselben Muster; Spielstand wird nicht zusätzlich in der Datenbank abgelegt.

Spielende (alle Modi, `pkg/domain/game/turn`): Nach dem ersten Checkout dürfen die übrigen Spieler der Runde nachziehen.
Das Spiel endet mit dieser Runde; alle, die darin auschecken, gewinnen gemeinsam (`winner_ids`).
X01 kennt die Out-Modi `single`, `double` und `master` (Doppel oder Triple).

## API

Alle Endpunkte unter `/api` außer `register`/`login`/`logout` brauchen das Session-Cookie aus `POST /api/login`.

- `POST /api/register` (`{"username", "name", "password"}`), `POST /api/login` (`{"username", "password"}`), `POST /api/logout`
- `GET /api/players` (eigener Spieler + Gäste), `POST /api/players` (`{"name"}`, legt einen Gast an)
- `GET /api/boards`, `POST /api/boards` (`{"name"}`), `DELETE /api/boards/{id}`
- `GET /api/games`, `POST /api/games`, z. B. `{"mode": "x01", "player_ids": [1, 2], "invite": ["ben"], "board_id": 1, "x01": {"start_score": 501, "out_mode": "double"}}`
  oder `{"mode": "around_the_clock", "player_ids": [1], "around_the_clock": {"bull": true, "multiplier_skips": false}}`; `player_ids` nur eigener Spieler und eigene Gäste; `invite` nur Freunde, sie kommen ans Ende der Wurfreihenfolge
- `GET /api/games/{id}`: Spiel, Spieler, Einladungen, Darts, berechneter Stand sowie `ready`, `is_owner`, `can_throw`, `invited`
- `POST /api/games/{id}/darts`: `{"number": 20, "multiplier": 3}` oder `{"miss": true}`; Bull ist `25`
- `DELETE /api/games/{id}/darts/last`: letzten Dart rückgängig machen
- `GET /api/friends` (Freunde, eingehende und gesendete Anfragen), `POST /api/friends` (`{"username"}`, nimmt eine bereits eingegangene Anfrage an), `POST /api/friends/{accountID}/accept`, `DELETE /api/friends/{accountID}` (entfernen, ablehnen oder zurückziehen)
- `GET /api/invitations`, `POST /api/games/{id}/accept`, `POST /api/games/{id}/decline`, `DELETE /api/games/{id}/invitations/{playerID}` (Gastgeber lädt aus)

## Testen

- Regeln und Service (Accounts, Rechte, Einladungen gegen In-Memory-SQLite): `go test ./pkg/...`
- Weboberfläche: mit [Lightpanda](https://github.com/lightpanda-io/browser) statt Headless Chrome, z. B. `lightpanda fetch <url> --inject-script-file klicks.js --wait-script '…' --dump semantic_tree_text --dump-selector '#board'`.
  Lightpanda führt JavaScript/htmx aus, rendert aber keine Pixel; für echte Screenshots gibt es damit keinen Ersatz.
