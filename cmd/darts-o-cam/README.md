# darts-o-cam (camera)

Detects darts with the cameras at your board and enters them into the running game on the [darts-o-mat server](../darts-o-mat/README.md). Part of [darts-o-mat](../../README.md).

The detection is still in progress, but already works for normal throws. It runs on macOS, Linux and Windows, on a desktop or headless (e.g. a Raspberry Pi next to the board), and needs no further programs. Tested with the three cameras of an Autodarts DIY kit.

## Requirements

- [Go](https://go.dev/dl/) is installed, the cameras are connected to this computer.
- **macOS:** started from the terminal, macOS asks on the first start whether the terminal may use the cameras. As an app of its own (Dock icon, double click, asks for "darts-o-cam"), build it with `make macapp` and start `darts-o-cam.app`; move it e.g. to `~/Applications`. After building it again, macOS may ask again.
- **Windows:** under Settings → "Privacy & security → Camera", camera access for desktop apps must be allowed. Only one program can use a camera at a time.
- **Linux:** packages (.deb, .rpm, .tar.gz for amd64 and arm64, e.g. a Raspberry Pi) can be built with `make linux` (into `dist/`). They add a menu entry and a service for running it without a desktop: `sudo systemctl enable --now darts-o-cam` (web interface on port 8090 for other devices). For the window of its own WebKitGTK must be installed, otherwise darts-o-cam opens the browser. Your user must be allowed to access the cameras (usually via the group `video`). If libjpeg-turbo is installed (Debian/Ubuntu/Raspberry Pi OS: `libturbojpeg0`, Arch: `libjpeg-turbo`), darts-o-cam needs considerably less computing power.

More about the supported systems: [camera support](../../pkg/camera/README.md).

## Start

From the project folder:

```sh
go run ./cmd/darts-o-cam
```

On a desktop the web interface opens in a window of its own; closing the window stops darts-o-cam. `-ui browser` opens it in the browser instead (<http://localhost:8090>). Everything else happens there, while darts-o-cam keeps running:

1. **Add the cameras:** under "Kamera hinzufügen" you see all cameras of this computer with a picture. Add the ones looking at the board. Not sure which is which? Hold a hand in front of one and take new pictures.
2. **Calibrate** each camera once, with an empty board: click the marked wire crossings on the double ring. Rough clicks are enough, the fine-tuning happens automatically. The detection uses the camera right away.
3. **Connect to darts-o-mat:** in darts-o-mat, create your board under "Boards" and click "Kamera-Token erzeugen". Enter the server address and the token in darts-o-cam under "Einstellungen".
4. Select the board when starting a game. Darts now appear by themselves.

Under "Einstellungen" you can also change the image size and frame rate and, for unusual setups, the thresholds of the detection; they are given in millimetres on the board and adapt to each camera's resolution and distance by themselves.

The overview shows what the cameras detected for the last throws (new ones appear by themselves) and whether the server took them, and lets you check ("Raster prüfen") whether the calibration still fits the board. If it warns that the grid doesn't fit, calibrate again. After plugging a camera in or out, click "Kameras neu verbinden".

Calibration and settings stay on this computer, so the same board works with any darts-o-mat server. They are kept in your user folder (macOS: `~/Library/Application Support/darts-o-cam`, Linux: `~/.config/darts-o-cam`, Windows: `%AppData%\darts-o-cam`); another folder can be chosen with `-dir`.

### Headless

Without a display (or over SSH) darts-o-cam opens no window and no browser. To use the web interface from another device, let it listen on the network:

```sh
go run ./cmd/darts-o-cam -addr :8090
```

Then open `http://<computer>:8090` on the other device. The web interface has no login: only do this in a network you trust.

## Debugging

`go run ./cmd/darts-o-cam -h` shows all options. Useful when something goes wrong:

- A detection looks wrong? Open it in the overview: under each camera, "Vorher" and "Nachher" download its whole images of that throw. The images of the last 20 throws are kept in your cache folder (macOS: `~/Library/Caches/darts-o-cam/events`, Linux: `~/.cache/darts-o-cam/events`, Windows: `%LocalAppData%\darts-o-cam\events`) and are cleared at the next start.
- `-v` logs every motion, `-save <folder>` keeps the camera images of every throw in that folder instead. These images can show people in the room – keep that in mind before sharing them.
- `go run ./cmd/darts-o-cam analyze -dir <folder>` explains what was detected in saved or downloaded images.
