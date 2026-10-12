# darts-o-cam (camera)

Detects darts with the cameras at your board and enters them into the running game on the [darts-o-mat server](../darts-o-mat/README.md). Part of [darts-o-mat](../../README.md).

The detection is still in progress, but already works for normal throws. It runs on macOS, Linux and Windows and needs no further programs. Tested with the three cameras of an Autodarts DIY kit.

## Requirements

- [Go](https://go.dev/dl/) is installed, the cameras are connected to this computer.
- **macOS:** on the first start macOS asks whether the terminal may use the cameras.
- **Windows:** under Settings → "Privacy & security → Camera", camera access for desktop apps must be allowed. Only one program can use a camera at a time.
- **Linux:** your user must be allowed to access the cameras (usually via the group `video`). If libjpeg-turbo is installed (Debian/Ubuntu/Raspberry Pi OS: `libturbojpeg0`, Arch: `libjpeg-turbo`), darts-o-cam needs considerably less computing power; at startup it then says "decoded with libjpeg-turbo".

More about the supported systems: [camera support](../../pkg/camera/README.md).

## Calibrate

Once, with an empty board. From the project folder:

```sh
go run ./cmd/darts-o-cam ui
```

1. Open <http://localhost:8090>.
2. Under "Kameras wählen", select the board's cameras and restart darts-o-cam.
3. Follow the instructions for each camera: click the marked wire crossings on the double ring. Rough clicks are enough, the fine-tuning happens automatically.

The calibration stays on this computer in `calibration.json`, so the same board works with any darts-o-mat server.

## Detect darts

In darts-o-mat, create your board under "Boards" and click "Kamera-Token erzeugen". The page shows a ready-made command, roughly:

```sh
go run ./cmd/darts-o-cam detect -server http://localhost:3000 -token <token>
```

Run it on the computer with the cameras and select the board when starting a game. Without `-server` darts-o-cam only prints the detected darts.

While it runs, the calibration interface stays available at <http://localhost:8090>: there you see what the cameras detected for the last throws and can check ("Raster prüfen") whether the calibration still fits the board. If it warns that the grid doesn't fit, calibrate again.

## Other commands

`go run ./cmd/darts-o-cam` without arguments lists all commands; `-h` after a command shows its options. Useful when something goes wrong:

- `detect -v` reports every motion, `detect -save <folder>` saves the camera images of every throw. These images can show people in the room – keep that in mind before sharing them.
- `analyze -dir <folder>` explains what was detected in saved images.
- `snapshot` saves one image per camera, `overlay` draws the calibrated board onto a camera image.
