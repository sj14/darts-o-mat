# TODO: darts-o-cam (camera)

Open tasks of the [camera](README.md). Tasks of the whole project: [TODO.md](../../TODO.md).

## Detection

- If not already implemented: stop image processing early when it is clear there is no dart or tip visible?
- Fuse the tips with the dart axes (needs recordings first, see [debug images](#debugging): darts close to wires with known fields). Each camera's dart axis, mapped onto the board plane, passes through the real entry point no matter where the changed area ends, so it is immune to where the changed area ends along the axis (the series of 2026-10-13 found no common bias there, but single cameras are off by ±2–3 mm); it is only exact across the axis. Intersecting the axes alone was tried and rejected (T4 only from 97.6 to 98.3 mm, one bad measurement pulled the bullseye 90 mm away: nearly parallel lines make the intersection unstable). Not tried yet: combine both in `FuseTips` as a weighted least-squares point – distance to the axis lines weighs strongly, the position along each axis (the detected tip) only weakly, so crossing axes remove the overshoot and nearly parallel ones fall back to about today's result; outlier cameras are dropped as today. Measure against the median on the recordings before keeping it.
- Find good detection thresholds automatically (optional, if the effort is reasonable; they can be set by hand under "Einstellungen"). Needs ground truth: e.g. a guided routine ("stick a dart into T20, then D5, …") whose saved frames are re-evaluated with different thresholds (like `darts-o-cam analyze`), keeping the values that score the most darts correctly. Moderate effort (routine in the UI plus a parameter search); simpler first step: re-evaluate saved events with different values to compare them.
  - A first series by hand (32 darts at wires, 2026-10-13) found no systematic error of the tip detection to correct, see [darts-o-cam → accuracy](AGENTS.md#accuracy); the routine would mainly help with the thresholds and with other setups.
- Check the detection with other setups (other cameras, resolutions, distances, number of cameras): the thresholds are in board millimetres now and scale per camera, but were only measured on the development board's three Autodarts cameras at 1280×720. Test first with own recordings in another resolution (e.g. 640×480); other camera models need recordings from such setups. Also check how fusion behaves with 1, 2 or more than 3 cameras.
- Frame size per camera, from the modes the camera offers: today there is one wanted size for all cameras (settings), and each camera uses its own mode closest to it. With different camera models, each camera should list its real modes and get its own size. Needs a mode list in `pkg/camera` for all three systems (macOS: the device's formats, Linux: `VIDIOC_ENUM_FRAMESIZES`, Windows: the native media types, which `start` already walks through), a size per camera in the configuration and a choice on the camera's card instead of the settings. About 150–250 lines, mostly only testable with cameras on Linux and Windows; only worth it once different camera models are used.

## Adding cameras

- Adding a camera: check the adaptive exposure settling (2026-10-16, `vision.settle` waits until the brightness stops changing, up to 6 s) with the webcam whose snapshot was much too dark after the fixed 1 s. If it still is, find out why (e.g. the camera's own exposure takes longer, or the lock); a live video instead of the snapshot was rejected for now (keeps every device open and encodes JPEGs while the page is open, and would hide a dark locked image during detection).

## Debugging

- Debug images: the frames of the last 20 detections are kept on disk (cache directory), can be downloaded on the event page and traced step by step ("Detection in detail"); `-save` keeps all. Maybe later: make the number configurable in the web interface.

## Testing on real systems

- Test the window of its own (glaze) on Linux (with and without WebKitGTK) and Windows.
- Start the Windows .exe (`make windows`) on Windows: icon in Explorer, taskbar and title bar, version under Properties → Details, sharp window with display scaling, no console window on a double click, log and Ctrl+C when started from a terminal (`console_windows.go`).
- Install the Linux packages (`tools/linuxpkg`) on a real system: .deb on Debian/Ubuntu and Raspberry Pi OS, .rpm on Fedora; menu entry and icon, window or browser fallback, the systemd service with the cameras (`DynamicUser` plus group `video`).

## Running on other systems

- Optimize for raspi with it's SD-Card so we won't destroy the card after using darts-o-cam for too long (too many writes).
- Keep the computer from going to sleep while the detection runs (nobody touches it while playing). [`github.com/crgimenes/native/power`](https://github.com/crgimenes/native/tree/trunk/power) (purego, macOS IOKit assertion, Windows `SetThreadExecutionState`) was rejected for now (user, 2026-10-13) because it doesn't support Linux; that would need systemd-logind's `Inhibit` over D-Bus.
- Maybe a container image (decided against for now, 2026-10-14; the server has one, see the root `Containerfile`). Only Linux can pass cameras to a container (`--device /dev/videoN --group-add video`; `pkg/camera` finds `/dev/video*` and reads `/sys/class/video4linux`, both present in a container); Docker on macOS and Windows runs in a VM without the USB cameras (Windows only via WSL and `usbipd`, doubtful at 3 × 30 fps MJPEG). Drawbacks: `--device` only covers the devices present at start, a replugged camera often gets another number and is gone until a restart (unless all of `/dev` is mounted with `--device-cgroup-rule='c 81:* rmw'`, which hardly isolates anything); only `-ui none`; the image needs glibc because of purego (e.g. `debian:stable-slim` with `libturbojpeg0`, not distroless static). The Linux packages with their systemd service cover the headless Raspberry Pi already.
