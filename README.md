# darts-o-mat

Ein Dart-Zähler für zu Hause: Würfe eintragen, Punkte werden automatisch gezählt.
Langfristig soll darts-o-mat eine freie Alternative zu Autodarts und Scolia werden, mit Kamera, die die Würfe selbst erkennt.

> **Stand:** frühe Entwicklung. Die Kamera-Erkennung gibt es noch nicht, Würfe werden von Hand eingetragen.

## Spielmodi

- **X01** (z. B. 301 oder 501): Von der Startpunktzahl auf genau 0 runterspielen. Zum Beenden je nach Einstellung beliebiger Dart, Doppel, oder Doppel/Triple („Master Out“).
- **Around the Clock**: Die Zahlen 1 bis 20 der Reihe nach treffen, auf Wunsch zum Schluss das Bull.

Wer als Erstes fertig ist, gewinnt, aber die anderen dürfen die Runde noch zu Ende werfen. Schaffen es mehrere, ist es ein Unentschieden.

## Starten

Voraussetzung: [Go](https://go.dev/dl/) ist installiert.

```sh
go run ./cmd/darts-o-mat
```

Dann im Browser <http://localhost:3000> öffnen. Die Daten landen in der Datei `darts-o-mat.db` im aktuellen Ordner.

### Mit PostgreSQL statt Datei

```sh
docker run -d --name darts-postgres -e POSTGRES_PASSWORD=geheim -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
go run ./cmd/darts-o-mat -db postgres -dsn "postgres://postgres:geheim@localhost:5432/darts-o-mat?sslmode=disable"
```

## Spielen

1. Beim ersten Besuch einen Account anlegen („Registrieren“). Jeder Account ist zugleich dein Spieler.
2. Mitspieler ohne Account legst du auf der Startseite als **Gastspieler** an. Wer einen eigenen Account hat, wird beim Spielstart per Benutzername **eingeladen** und muss zusagen; bis alle zugesagt haben, wartet das Spiel.
3. Spielmodus wählen, Mitspieler ankreuzen bzw. einladen und starten.
4. Nach jedem Wurf eintragen: erst Single, Doppel oder Triple wählen, dann die Zahl antippen. Für das Bull „Bull“ (mit „Doppel“ = Bullseye), für Fehlwürfe „Daneben“. Alle Mitspieler mit Account können auf ihrem eigenen Gerät mit eintragen.
5. Vertippt? „Letzten Dart zurücknehmen“.

Unter „Boards“ kannst du deine Dartboards anlegen (z. B. Keller, Verein) und beim Spielstart auswählen.

Läuft darts-o-mat öffentlich erreichbar, kannst du die Registrierung neuer Accounts abschalten: `go run ./cmd/darts-o-mat -registration=false`.
