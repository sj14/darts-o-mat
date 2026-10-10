# darts-o-mat

## Projektziel

Ziel des Projekts ist es, eine Alternative zu [Autodarts](https://autodarts.io) und [Scolia](https://scoliadarts.com) zu schaffen.

## Dokumentation

`README.md` richtet sich ausschließlich an Menschen: kurz, übersichtlich und ohne Vorwissen verständlich (was ist das, wie starte ich es, wie probiere ich es aus).
Keine Implementierungsdetails, Architektur oder Anweisungen für Agents in die README; die gehören in diese Datei.
Ändert sich etwas, das Nutzer betrifft (Start, Spielmodi, Bedienung), die README mit anpassen.

## Technologien

Neue Funktionen bauen auf dem bestehenden Stack auf; ohne Absprache keine zusätzlichen Frameworks oder Bibliotheken einführen.

- **Sprache:** Go (Version siehe `go.mod`)
- **HTTP-Routing:** [chi](https://github.com/go-chi/chi) v5
- **Frontend:** serverseitig gerenderte HTML-Templates mit [templ](https://templ.guide) (als Go-Tool in `go.mod` eingebunden; generieren mit `go tool templ generate`) und [htmx](https://htmx.org) für Interaktivität: Handler liefern templ-Fragmente, die htmx in die Seite tauscht, statt eigenem JavaScript bzw. einem JS-Framework
- **Datenbank:** PostgreSQL und SQLite werden beide unterstützt (Auswahl per `-db sqlite|postgres` und `-dsn`). Persistenz-Code muss mit beiden Datenbanken funktionieren.
- **ORM/Query-Builder:** [Bun](https://bun.uptrace.dev) (`pgdialect`/`pgdriver` für PostgreSQL, `sqlitedialect` mit dem cgo-freien Treiber `modernc.org/sqlite` für SQLite)
- **Messaging (optional):** [NATS](https://nats.io) darf aufgenommen werden, wenn es gebraucht wird, z. B. um Würfe von der Kamera (`darts-o-cam`) an den Server (`darts-o-mat`) zu übertragen.

## Struktur

- `cmd/darts-o-mat`: Webserver
- `cmd/darts-o-cam`: Kamera-Client, der Würfe erkennt und an den Server meldet
- `pkg/api`: JSON-API unter `/api`
- `pkg/domain`: Fachlogik (Board, Spieler, Würfe, Spielmodi X01 und Around the Clock)
- `pkg/frontend`: Controller und templ-Views
- `pkg/repos/store`: Persistenz; ein gemeinsamer Bun-Store für beide Datenbanken, nur der Verbindungsaufbau (`NewPostgres`/`NewSQLite`) unterscheidet sich

## Spielstand

Gespeichert werden nur das Spiel (Modus, Einstellungen, Spieler in Wurfreihenfolge) und die geworfenen Darts.
Der aktuelle Stand wird immer durch Abspielen der Darts berechnet (`game.Game.Play`, je Modus eine `Play`-Funktion).
Neue Spielmodi folgen demselben Muster; Spielstand wird nicht zusätzlich in der Datenbank abgelegt.

Spielende (alle Modi, `pkg/domain/game/turn`): Nach dem ersten Checkout dürfen die übrigen Spieler der Runde nachziehen.
Das Spiel endet mit dieser Runde; alle, die darin auschecken, gewinnen gemeinsam (`winner_ids`).
X01 kennt die Out-Modi `single`, `double` und `master` (Doppel oder Triple).

## API

- `GET /api/players`, `POST /api/players` (`{"name": "…"}`)
- `POST /api/games`, z. B. `{"mode": "x01", "player_ids": [1, 2], "x01": {"start_score": 501, "out_mode": "double"}}`
  oder `{"mode": "around_the_clock", "player_ids": [1], "around_the_clock": {"bull": true, "multiplier_skips": false}}`
- `GET /api/games/{id}`: Spiel, Darts und berechneter Stand
- `POST /api/games/{id}/darts`: `{"number": 20, "multiplier": 3}` oder `{"miss": true}`; Bull ist `25`
- `DELETE /api/games/{id}/darts/last`: letzten Dart rückgängig machen
