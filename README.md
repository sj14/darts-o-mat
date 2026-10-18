# darts-o-mat

A free, open-source darts scorer that you host yourself: enter your throws or let cameras at the board detect them, the score is counted for you. X01 and Around the Clock, legs and sets, tournaments, friends, guest players and statistics, in English and German.

> [!NOTE]
> This project was build with AI.

> **Status:** early development.

![The game page with the darts the cameras located on the board](site/screenshots/game-board-en.png)

<p>
  <img src="site/screenshots/game-keypad-en.png" alt="The game page with the keypad" width="49%">
  <img src="site/screenshots/tournament-en.png" alt="A knockout tournament" width="49%">
</p>
<p>
  <img src="site/screenshots/stats-en.png" alt="Statistics" width="49%">
  <img src="site/screenshots/game-mobile-en.jpg" alt="The game page on a phone" width="24%">
</p>

## What's in it

- **[darts-o-mat](cmd/darts-o-mat/README.md)** – the server: a web app for playing, with accounts, friends, guest players, tournaments, games with others (also with strangers) and statistics. This is all you need to play by hand.
- **[darts-o-cam](cmd/darts-o-cam/README.md)** – the camera program: runs on the computer the board's cameras are connected to, detects the darts and enters them into the running game, as an app with a window for macOS, Linux and Windows without further programs ([details on camera support](pkg/camera/README.md)).

## Quick start

The [releases](https://github.com/sj14/darts-o-mat/releases) have the server for Linux, macOS and Windows and darts-o-cam as a macOS app, Linux packages (.deb, .rpm) and a Windows zip. Start the server and open <http://localhost:3000>:

```sh
./darts-o-mat
```

Or run it in a container (the games are kept in the volume):

```sh
docker run -d -p 3000:3000 -v darts-o-mat:/data ghcr.io/sj14/darts-o-mat
```

Or on Kubernetes with the [Helm](https://helm.sh) chart:

```sh
helm install darts oci://ghcr.io/sj14/charts/darts-o-mat
```

From the source, with [Go](https://go.dev/dl/) installed: `go run ./cmd/darts-o-mat`.

Then create an account ("Register"). How to play and more options (PostgreSQL, turning off registration): [darts-o-mat](cmd/darts-o-mat/README.md).

## Playing with cameras

1. Start darts-o-cam on the computer the cameras are connected to, add the cameras and calibrate them once: [darts-o-cam → start](cmd/darts-o-cam/README.md#start).
2. In darts-o-mat, create your board under "Boards" and click "Create camera token".
3. Enter the server address and the token in darts-o-cam under "Settings".
4. Select this board when starting a game. The darts now appear by themselves and the keypad gives way to a large board. If a dart is missed, add it at the next free place of the visit; click a dart of yours to correct it.

On a board the next player is only up once the darts have been pulled out – the camera notices that (if it misses it, the next player's first dart counts as pulled out); without a camera, tap "Darts pulled out". If a dart of the visit is missing (e.g. bounced out), it counts as a miss.

Each player can throw at a board of their own, also when playing with friends or strangers from different places: everybody picks their board when joining.

## For developers

Notes on architecture and decisions are in the `AGENTS.md` files: [project](AGENTS.md), [server](cmd/darts-o-mat/AGENTS.md), [camera](cmd/darts-o-cam/AGENTS.md), [camera access](pkg/camera/AGENTS.md). Run the tests with `go test ./...`. Planned work: [project](TODO.md), [server](cmd/darts-o-mat/TODO.md), [camera](cmd/darts-o-cam/TODO.md), [camera access](pkg/camera/TODO.md).

## License

[AGPL-3.0](LICENSE), except the camera access in [`pkg/camera`](pkg/camera), which is [MIT](pkg/camera/LICENSE).
