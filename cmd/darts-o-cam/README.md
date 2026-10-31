# darts-o-cam (camera)

Detects darts with the cameras at your board and enters them into the running game on the [darts-o-mat server](../darts-o-mat/README.md). Part of [darts-o-mat](../../README.md).

The detection is still in progress, but already works for normal throws. It runs on macOS, Linux and Windows, on a desktop or headless (e.g. a Raspberry Pi next to the board), and needs no further programs. Tested with the three cameras of an Autodarts DIY kit. Its interface is in English or German, following your browser (or, in its own window, your system).

## Requirements

- The cameras are connected to this computer. Download darts-o-cam from the [latest release](https://github.com/sj14/darts-o-mat/releases/latest): [macOS app](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_macos_arm64.zip) ([Intel](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_macos_amd64.zip)), [Windows](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_windows_amd64.zip) ([ARM](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_windows_arm64.zip)), Linux as [.deb](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_amd64.deb) ([ARM](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_arm64.deb)), [.rpm](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_amd64.rpm) ([ARM](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_arm64.rpm)) or [.tar.gz](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_amd64.tar.gz) ([ARM](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-cam_linux_arm64.tar.gz)). (Without a note: Intel/AMD processors, ARM is e.g. a Raspberry Pi; macOS: Apple silicon.) On macOS and Linux one line in the terminal does it for you: it picks the right package for your computer, installs it and, run again later, updates it:

  ```sh
  curl -fsSL https://raw.githubusercontent.com/sj14/darts-o-mat/main/install.sh | bash
  ```

  (`bash install.sh --dry-run` only shows what it would do, `--version 1.2.3` takes another release; it checks the download against the release's checksums.) To build it yourself, [Go](https://go.dev/dl/) has to be installed (see the commands below).
- **macOS:** started from the terminal, macOS asks on the first start whether the terminal may use the cameras. As an app of its own (Dock icon, double click, asks for "darts-o-cam"), unpack the .zip from the release and move `darts-o-cam.app` e.g. to `~/Applications`, or build it with `make macapp`. The app is not signed: macOS warns on the first start (right click → Open, or `xattr -dr com.apple.quarantine darts-o-cam.app`). After a new version macOS may ask for camera access again.
- **Windows:** under Settings → "Privacy & security → Camera", camera access for desktop apps must be allowed. Only one program can use a camera at a time. The .zip of the release (or `make windows`, into `dist/`) has `darts-o-cam.exe` with its icon: unpack it anywhere and double click it. As it is unsigned, Windows warns on the first start ("More info" → "Run anyway").
- **Linux:** packages (.deb, .rpm, .tar.gz for amd64 and arm64, e.g. a Raspberry Pi) are on the release page, linked above (or build them with `make linux`, into `dist/`). They add a menu entry and a service for running it without a desktop: `sudo systemctl enable --now darts-o-cam` (web interface on port 8090 for other devices). For the window of its own WebKitGTK must be installed, otherwise darts-o-cam opens the browser. Your user must be allowed to access the cameras (usually via the group `video`). libjpeg-turbo, which the .deb and .rpm bring along, makes darts-o-cam need considerably less computing power; for the .tar.gz install it yourself (Debian/Raspberry Pi OS: `libturbojpeg0`, Ubuntu/Linux Mint: `libturbojpeg`, Fedora: `turbojpeg`, Arch: `libjpeg-turbo`).

More about the supported systems: [camera support](../../internal/cam/camera/README.md).

## Start

Start the app (macOS: double click, Windows: the .exe, Linux: the menu entry or `darts-o-cam`). From the source, from the project folder:

```sh
go run ./cmd/darts-o-cam
```

On a desktop the web interface opens in a window of its own; closing the window stops darts-o-cam. `-ui browser` opens it in the browser instead (<http://localhost:8090>). Everything else happens there, while darts-o-cam keeps running:

1. **Add the cameras:** under "Add camera" you see all cameras of this computer with a picture. Add the ones looking at the board. Not sure which is which? Hold a hand in front of one and take new pictures.
2. **Calibrate** each camera once, with an empty board: click once into the narrow outer ring (the double) of the field 20, the rest is found automatically. If that doesn't work, darts-o-cam says so and asks for the marked wire crossings on the double ring instead; rough clicks are enough, the fine-tuning happens automatically. The detection uses the camera right away.
3. **Connect to darts-o-mat:** in darts-o-mat, create your board under "Boards" and click "Create camera token". Enter the server address and the token in darts-o-cam under "Settings".
4. Select the board when starting a game. Darts now appear by themselves.

**Several boards on one computer:** under "Settings" click "Add board", give it a name, its own token (and server) and add its cameras under "Add camera" (choose the board there). With more than one board the pages show a tab per board; each board has its own game, detection, detections list and settings (image size, frame rate, thresholds).

Under "Settings" you can also change, per board, the image size and frame rate and, for unusual setups, the thresholds of the detection; they are given in millimetres on the board and adapt to each camera's resolution and distance by themselves.

The overview shows what the cameras detected for the last throws (new ones appear by themselves) and whether the server took them, and lets you check ("Check the grid") whether the calibration still fits the board. If it warns that the grid doesn't fit, calibrate again. After plugging a camera in or out, click "Reconnect cameras". During a game, the game page in darts-o-mat warns if a camera drops out, the detection stops, a camera delivers only small images or a camera was moved and needs calibrating again. If a camera's image is much smaller than the others, the overview says so: plug that camera into another USB port or directly into the computer, or try another cable, and click "Reconnect cameras" again.

Calibration and settings stay on this computer, so the same board works with any darts-o-mat server. They are kept in your user folder (macOS: `~/Library/Application Support/darts-o-cam`, Linux: `~/.config/darts-o-cam`, Windows: `%AppData%\darts-o-cam`); another folder can be chosen with `-dir`.

### Headless

Without a display (or over SSH) darts-o-cam opens no window and no browser. To use the web interface from another device, let it listen on the network:

```sh
go run ./cmd/darts-o-cam -addr :8090
```

Then open `http://<computer>:8090` on the other device. The web interface has no login: only do this in a network you trust.

## Updates

The footer of the interface says "Version … available" when there is a newer release (darts-o-cam asks GitHub for the latest release when it starts and then once a day; GitHub sees your address and the version; started with `-no-update-check` it doesn't ask). Install it like the first time; with the installer line above, run it again. Your cameras and settings are kept.

## Debugging

`go run ./cmd/darts-o-cam -h` shows all options. Useful when something goes wrong:

- A detection looks wrong? Open it in the overview: "Detection in detail" shows step by step what each camera found – the changed pixels, the dart, its axis and the tip – drawn into its image, with a zoom around the tip. Under each camera, "Before" and "After" download its whole images of that throw. The images of the last 5 throws are only kept while darts-o-cam runs, nothing is written to disk (good for a Raspberry Pi's SD card).
- `-v` logs every motion, `-save <folder>` keeps the camera images of every throw in that folder, as PNG (exact, larger). These images can show people in the room – keep that in mind before sharing them.
- `go run ./cmd/darts-o-cam analyze -dir <folder>` explains what was detected in saved or downloaded images.
- `-pprof localhost:6060` serves Go's profiles (CPU, memory, goroutines) for `go tool pprof http://localhost:6060/debug/pprof/profile`, to see where the detection spends its time. Keep it on `localhost`: there is no login.
