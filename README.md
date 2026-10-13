# darts-o-mat

A darts scorer for home: enter your throws, the score is counted automatically.
In the long run darts-o-mat aims to be a free alternative to Autodarts and Scolia, with cameras that detect the throws by themselves.

> **Status:** early development. Darts are entered by hand or detected by cameras at the board; the camera detection already works for normal throws but is still in progress. The user interface is in German.

## What's in it

- **[darts-o-mat](cmd/darts-o-mat/README.md)** – the server: a web app for playing (X01 and Around the Clock), with accounts, friends, guest players and statistics. This is all you need to play by hand.
- **[darts-o-cam](cmd/darts-o-cam/README.md)** – the camera program: runs on the computer the board's cameras are connected to, detects the darts and enters them into the running game. Works on macOS, Linux and Windows without further programs ([details on camera support](pkg/camera/README.md)).

## Quick start

Requirement: [Go](https://go.dev/dl/) is installed.

```sh
go run ./cmd/darts-o-mat
```

Then open <http://localhost:3000> in the browser and create an account ("Registrieren"). How to play and more options: [darts-o-mat](cmd/darts-o-mat/README.md).

## Playing with cameras

1. Start darts-o-cam on the computer the cameras are connected to, add the cameras and calibrate them once: [darts-o-cam → start](cmd/darts-o-cam/README.md#start).
2. In darts-o-mat, create your board under "Boards" and click "Kamera-Token erzeugen".
3. Enter the server address and the token in darts-o-cam under "Einstellungen".
4. Select this board when starting a game. Darts now appear by themselves; correct misdetections as usual.

On a board the next player is only up once the darts have been pulled out – the camera notices that; without a camera, tap "Pfeile gezogen". If a dart of the visit is missing (e.g. bounced out), it counts as a miss.

## For developers

Notes on architecture and decisions are in the `AGENTS.md` files: [project](AGENTS.md), [server](cmd/darts-o-mat/AGENTS.md), [camera](cmd/darts-o-cam/AGENTS.md), [camera access](pkg/camera/AGENTS.md). Run the tests with `go test ./...`. Planned work: [TODO.md](TODO.md).
