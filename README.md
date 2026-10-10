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

Danach läuft darts-o-mat auf Port 3000. Die Daten landen in der Datei `darts-o-mat.db` im aktuellen Ordner.

### Mit PostgreSQL statt Datei

```sh
docker run -d --name darts-postgres -e POSTGRES_PASSWORD=geheim -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
go run ./cmd/darts-o-mat -db postgres -dsn "postgres://postgres:geheim@localhost:5432/darts-o-mat?sslmode=disable"
```

## Ausprobieren

Eine Bedienoberfläche ist noch im Aufbau. Bis dahin lässt sich ein Spiel z. B. mit `curl` spielen:

```sh
# Spieler anlegen
curl -X POST localhost:3000/api/players -d '{"name": "Anna"}'

# 501-Spiel mit Double Out starten
curl -X POST localhost:3000/api/games -d '{"mode": "x01", "player_ids": [1], "x01": {"start_score": 501, "out_mode": "double"}}'

# Triple 20 werfen
curl -X POST localhost:3000/api/games/1/darts -d '{"number": 20, "multiplier": 3}'

# Letzten Wurf zurücknehmen
curl -X DELETE localhost:3000/api/games/1/darts/last
```
