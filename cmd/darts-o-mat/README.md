# darts-o-mat (server)

The web app for playing darts: enter throws, the score is counted automatically. Part of [darts-o-mat](../../README.md); throws can also come from cameras via [darts-o-cam](../darts-o-cam/README.md).

## Game modes

- **X01** (e.g. 301 or 501): play down from the start score to exactly 0. To finish, depending on the setting any dart, a double, or a double/triple ("Master Out"). The game page suggests a checkout.
- **Around the Clock**: hit the numbers 1 to 20 in order, optionally the bull at the end. The next target lights up on the dartboard of the game page.

Whoever finishes first wins the leg immediately. When starting a game you choose how many legs are needed to win ("First to 3"); each leg is started by the next player. Optionally the legs are played in sets ("First to 3 Sets à 3 Legs"): whoever wins the legs of a set first wins the set, and whoever wins enough sets wins the game; each set is started by the next player. Who starts the first leg can be decided by a bull-off (option when starting a game): everyone throws one dart at the bull, the closest starts – with cameras to the millimeter.

## Tournaments

Every game mode can also be played as a tournament: under "Spielform" on the start page choose a knockout tournament ("K.-o.-Turnier") or all against all ("Jeder gegen jeden"), with 3 to 32 players – you, your guests and invited friends. The order is drawn at random. On the tournament's page you start the games one after another; the bracket or the table fills in by itself, and below you see everyone's statistics in this tournament. Invited friends accept once for the whole tournament.

## Start

Requirement: [Go](https://go.dev/dl/) is installed. From the project folder:

```sh
go run ./cmd/darts-o-mat
```

Then open <http://localhost:3000> in the browser. The data is stored in the file `darts-o-mat.db` in the current folder.

Options:

- `-addr :8080` – listen on another port
- `-registration=false` – turn off registering new accounts, e.g. if darts-o-mat is publicly reachable

### In a container

Instead of Go, [Docker](https://www.docker.com) (or Podman) is enough. From the project folder:

```sh
docker build -f Containerfile -t darts-o-mat .
```

```sh
docker run -d -p 3000:3000 -v darts-o-mat:/data darts-o-mat
```

The data is kept in the volume `darts-o-mat`. Options go after the image name, e.g. `docker run … darts-o-mat -registration=false`.

### With PostgreSQL instead of a file

```sh
docker run -d --name darts-postgres -e POSTGRES_PASSWORD=geheim -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
```

```sh
go run ./cmd/darts-o-mat -db postgres -dsn "postgres://postgres:geheim@localhost:5432/darts-o-mat?sslmode=disable"
```

## Playing

1. On your first visit create an account ("Registrieren"). Every account is also your player.
2. Add fellow players without an account on the start page as **guest players**. Players with their own account are added under **Freunde** by username. Once they accept the request, you can **invite** them when starting a game; they have to accept, until then the game waits.
3. Choose the game mode, tick or invite your fellow players and start.
4. Enter every throw: first choose single, double or triple, then tap the number – the keys then read e.g. "D20" or "T20". For the bull "Bull" (with "Doppel" it becomes "Bullseye"; there is no triple bull), for misses "Daneben". Misses detected by the cameras show where they went, e.g. "M7" beside the 7. All players with an account can enter throws on their own device.
5. Mistyped or misdetected? Tap the dart in the running visit or in the history and choose what was really hit. "Letzten Dart zurücknehmen" undoes the last dart.
6. The game page plays sounds for hits, misses, busts and 180; toggle them at the top with "🔊 Ton an". Browsers only allow sounds once you have clicked or tapped the page. If the top says "🔈 Für Ton klicken", click it – important when the cameras enter the throws and you don't touch anything else.
7. Had enough? Whoever created the game can end it with "Spiel abbrechen". It stays in the list but doesn't count in the statistics.

During the game you see the average or hit rate and a small statistic for every player. The history shows every visit with its darts. Under "Statistik" you find your values and those of your guest players over all finished games.

Under "Boards" you can create your dartboards (e.g. basement, club) and select them when starting a game; the board you used last is preselected. To have darts detected automatically, see [playing with cameras](../../README.md#playing-with-cameras).
