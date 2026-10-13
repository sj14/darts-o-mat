# darts-o-cam (camera)

Detects darts with classical image processing in pure Go (`pkg/vision`) and reports them to darts-o-mat (server and token from its settings; otherwise it only shows them).
Project-wide rules, the shared stack and how server and camera work together: [root AGENTS.md](../../AGENTS.md). For humans: [README.md](README.md). Camera access per operating system: [`pkg/camera`](../../pkg/camera/AGENTS.md). The server side: [darts-o-mat](../darts-o-mat/AGENTS.md).

## Structure

- `cmd/darts-o-cam`: starts the app with its web interface (flags `-dir`, `-addr`, `-open`, `-v`, `-save`), opens the browser on a desktop (`browser.go`); the only other command is `analyze` (re-evaluate events saved with `-save`).
- `pkg/camapp`: the app at runtime: configuration, the open cameras, the detection run, events, reporting to the server (`server.go`). Logic belongs here, not into the handlers.
- `pkg/camui`: its web interface (cameras, add camera, calibrate, check the grid, settings, latest detections). templ + Tailwind like the server, see [root AGENTS.md → technologies](../../AGENTS.md#technologies).
- `pkg/vision`: camera geometry and dart detection in pure Go (homography, calibration, image comparison, watcher, opening and assigning the cameras of a calibration)
- [`pkg/camera`](../../pkg/camera/AGENTS.md): camera access per operating system (list devices, open, read frames, lock exposure)
- `pkg/domain/dartboard`: board geometry (standard dimensions in mm, origin at the bull, Y points towards the 20; `Score` scores a position), shared with the server

## App

One program for desktop and headless (user's wish, 2026-10-13): `darts-o-cam` without arguments starts the web interface and keeps running; everything is set up there, without restarts or flags.

- **Configuration** (`camapp.Config`, `config.json`): the cameras with their calibration (`vision.CameraCalibration`), server URL and token, `detect` (on/off, kept for the next start), size and fps. It lives in the user's config directory (`os.UserConfigDir()/darts-o-cam`, `-dir` for another one) next to the cameras' images (reference and empty board per camera), written atomically with mode 0600 because of the token. The calibration stays with the client (user's decision, see [root AGENTS.md](../../AGENTS.md#how-server-and-camera-work-together)).
- **Every change takes effect immediately:** the app keeps the cameras open (`vision.Live`, shared by the detection and the web interface) and starts the detection over (`App.restart`) after a calibration, a camera added or removed, or new settings. A run takes everything it needs (cameras, homographies, empty images, server) when it starts and never takes `App.mu`, so stopping it can wait for it; HTTP requests to the server use its context. A new size or fps opens all cameras again.
- **Which cameras the detection uses:** open, calibrated for the size of their frames (`CameraCalibration.Width`/`Height`; a calibration is in pixels) and without problem. A camera whose view doesn't match its reference image (`vision.ErrAmbiguousCameras`) may be another camera of the same name with the wrong calibration: it is opened (to calibrate it again) but not used until then.
- **Adding cameras:** "Kamera hinzufügen" lists all devices (`camera.Devices`) with a still image each – of a board camera from its live frames, of other devices taken by opening them for a moment (`App.Snapshot`, cached until "Neue Bilder aufnehmen", e.g. to hold a hand in front of a camera). Adding opens the device (`vision.OpenCamera`), stores its device name, model and the frame as reference image; it is named `cam1`, `cam2`, … (first free) and the web interface goes on to calibrate it. Removing deletes its calibration and images. "Kameras neu verbinden" opens all cameras again after one was plugged in or out.
- **Browser and headless:** on a desktop (macOS/Windows, Linux with `DISPLAY`/`WAYLAND_DISPLAY`, not over SSH) the browser opens (`open`, `xdg-open`, `rundll32`); `-open` overrides it. The web interface listens on `localhost:8090`; `-addr :8090` makes it reachable from other devices, e.g. a headless Raspberry Pi – it has no login, so this is opt-in.

## Calibration interface

In the web interface (`pkg/camui`):

- One still image per camera, the user clicks wire crossings at the double ring one after another (`camui.Targets`, hidden ones can be skipped, at least 4; click position via `<input type="image">`, image in original size, no custom JavaScript), then automatic fine calibration on the same image (`camapp.App.Calibrate` stores it with the image as reference and empty board). "Nur feinkalibrieren" refines again on the current frame with the clicked points, e.g. after a camera was bumped.
- Rough clicks are enough: in the test 6 px off → 0.33 mm deviation from the careful calibration.
- The overview shows the last 20 detections (per camera an image crop with the tip, a board sketch as SVG with the camera positions and the result, and whether the server entered it; "Vorher"/"Nachher" download each camera's whole frames, named like `-save` writes them, e.g. `028-cam3-before.jpg`, JPEG quality 95, so `analyze` reads them) and "Raster prüfen": fine calibration on the current image without saving, with the found ring edges and wires colored by deviation (`RefineResult.Features`).
- If the fine calibration finds less than half of the searched edges and wires (`RefineResult.Searched`, 656 places; `minFoundShare`), the page warns that the grid doesn't fit, with a button "Neu kalibrieren"; after calibrating you land right there. Reason (2026-10-12): a wire was mixed up while clicking (1/18 instead of 20/1; with 4 points the homography always works out, so it doesn't show while clicking), the fine calibration then refined a rotated grid with a harmless mean deviation (1.7 px). Found: correct 97–98 %, rotated 22 %. The share of edges > 2 px off is no good criterion (built that way first): correct 4–16 %, varies from image to image, rotated 25–35 %.
- **Not implemented (deliberately):** recalibrating from corrections in the game. A correction only names the field, not the position, and the errors demonstrably lie in finding the tip, not in the calibration (~0.3 mm). Open proposal to the user: automatically fine-calibrate after every takeout with an empty board (compensates for shifted cameras) – only after their decision, because this changes the calibration file automatically.

## Connection to the game

With server and token set (settings page), the detection sends every dart (with the fused position in mm) to `POST /api/board/darts` and every takeout to `POST /api/board/takeout`, with `Authorization: Bearer <token>`. What the server does with them (which game, missing darts on takeout, errors): [darts-o-mat → camera connection](../darts-o-mat/AGENTS.md#camera-connection).

## Calibration

In two steps:

1. Roughly by hand: per camera at least 4 image points where the outer edge of the double ring crosses a wire (`between: [a, b]` clockwise); from them a homography image → board. To find the points: crops with a grid (e.g. with ffmpeg, only as a viewing tool), cross-check via the ring colors (double/triple of the 20 red, of the 1 green, alternating). Rough is enough: the fine calibration gets from 15–34 mm off to within 0.3 mm of the same result (`TestRefineCalibrationReal`).
2. Automatically fine (`vision.RefineCalibration`, run by the web interface on the clicked image) with an image of the **empty** board per camera: around 600 features – edges of the colored rings as a jump in color saturation (rings red/green, fields black/white), wires as a change red↔green along the triple and double ring. Only the distance across the respective edge is fitted (point-to-line, Gauss-Newton); treating an edge as a full point pulled the result back to the starting calibration. Search window per ring limited to < half the distance to the neighboring edge (otherwise the search jumps to the wrong edge at the small bull). The result is stored as `refined` in the calibration and takes precedence over the points. Residual deviation in reality ~1.1–1.4 px (synthetic 0.3 px): edge noise on sisal/wires, **not** lens distortion (a radial distortion term only reduced it by 6–8 %, therefore not built in).

The web interface draws the computed grid onto each camera's image ("Raster prüfen" also shows the edges found); the calibration only counts once it visibly fits. Calibration of the development board: `pkg/vision/testdata/calibration.json` (reference images and images of the empty board next to it). Pixel centers in `vision` lie on whole numbers.

## Detection

**Thresholds in board millimetres** (2026-10-13, so other cameras, resolutions and distances work): `DetectOptions`/`WatchOptions` give sizes in mm and mm²; the watcher converts them per camera with its scale (`vision.PixelScale`: square root of the board's area inside the double ring in the image divided by its area in mm², i.e. the mean over the near and far side), rounding lengths and areas to whole pixels. The defaults are the pixel values measured on the development board divided by `vision.ReferenceScale` (1.4067 px/mm, mean of its cameras: 1.435/1.414/1.371), so at that scale they are exactly the old pixel values (`TestDefaultsInPixels`); on the development board the cameras' own scales shift only areas a little (min. size 142–156 instead of 150 px, motion 38–42 instead of 40 samples), `TestRealThrows` runs with each camera's scale. Not scaled: color thresholds, elongation, and the pixel details of finding the tip (skip the 3 outermost pixels, center of the last 3 px), which are about pixel noise, not the dart's size. The thresholds can be changed under "Einstellungen → Erkennung (für Fortgeschrittene)" (`camapp.Config.Detection`, checked by `validDetection`); the overview shows each camera's px/mm. Not measured yet on another setup.

`vision.DetectDart` (pixel values at the reference scale): color difference to the image before the throw (grayscale isn't enough: a silver dart on a red/green bull is equally bright), 5×5 blurred, hysteresis thresholds 25/12, blobs < 15 px discarded.

- The change of real darts falls apart into pieces (dark shaft in front of a black field separates flight and barrel; barrel in front of black fields only dashed), therefore: merge pieces with gaps up to 6 px; the starting point is the largest piece whose **lower half is elongated** (barrel and tip; the flight, large from close up, would distort axis and length-width ratio); pieces on its axis that start above its lower end are added (the flight), blobs next to it aren't, and neither are pieces below it: there is board (2026-10-13: color noise at the triple ring's edge 56 px below a real tip, on the axis, became the tip and turned a miss outside D5 into a 12, twice at the same spot; `TestDetectIgnoresSpeckBelowTip`, `testdata/throws/006-cam2`). No real throw had a piece of the dart below the seed; small gaps in the point are bridged anyway. The starting piece itself always belongs to the dart (with a big flight its center can be off the axis of its lower half, which dropped it entirely).
- The tip is the **lower** end along this axis (the cameras look flat across the board from the edge, darts point upwards). Synthetic darts: tip accurate to ≤ 5 px (the blur washes out the last pixels of the thin tip).
- Extending the tip along the axis in the unblurred image helped with synthetic darts, but with real ones ran into the shadow of the tip and worsened the results (measured on 5 real throws) – don't build it in again without a new measurement.
- If a camera only sees the flight (rest hidden or black on black), it reports no tip instead of guessing.

## Fusion

Convert each camera's tip onto the board, discard results more than 225 mm from the center (`MaxBoardRadius`, e.g. a reflection in the image corner), then median of the distance to the center and median of the angle (`FuseTips`; below 31.8 mm distance, where the cameras surround the dart, median per coordinate).

- The median per coordinate alone combined x of one camera with y of another: a real T18 (cam1 99.6 mm, cam3 100.2 mm, cam2 96.3 mm) became 98.0 mm and S18 that way (`TestFuseTips`). This also solves the T4 case below (median 99.0 instead of 97.6 mm).
- Afterwards a camera more than 20 mm away from the result is dropped (`maxCameraDeviation`; correct cameras were up to 16 mm apart), and the rest are recombined – with two, their mean instead of the one closer to the outlier (`TestFuseTipsDropsOutlier`, real T13 next to a stuck dart: cam3 saw the new dart only half behind the old one and was 35 mm off; cam1 97.2 mm, cam2 100.8 mm → 99.0 mm, so just T13 – luck, the actual cause is the tip detection, see below).
- A camera that doesn't see the entry point (dart points away from it, compressed image edge) was 15 mm off in the test; the median catches that.

## Flow

`vision.Watcher`: motion over every 4th pixel with threshold 40 (camera noise ≤ 5 samples, a dart ~160), wait until 3 frames are calm (at 10 fps 0.3 s), then evaluate. The evaluation runs only once per camera (`detect` returns the tip and the changed area for the takeout check) and for all cameras in parallel; the blur works with running sums (`TestBlur` checks against direct averaging). Measured: 3 cameras in 0.10 s instead of 1.85 s (previously twice per camera, one after another, blur re-summed per pixel). Very large changes or no recognizable tip count as takeout (`takeout`).

**Detecting the takeout:** the image comparison is symmetric – a disappeared dart looks like a new one. Therefore the watcher compares the changed spot with the image of the **empty board** (`empty` in the calibration, stored when calibrating): if it looks more like the empty board afterwards than before, darts were pulled (`Watcher.removed`). Measured on 11 real events: takeout ~0.05 (0.55 if a neighboring dart was moved along), throws 4–17, threshold 1.

- Each camera votes individually, the majority decides (tie: sum): a camera that readjusts its exposure during the takeout (cam2 on the development board, hand in front of the lens) changes almost in the whole image and outvoted the others in the sum (real takeout 0.06/1.9/0.08 → sum 1.19, counted as throw; `TestWatcherRemovedExposure`, `testdata/exposure`). The vote also evaluated a double throw (empty → 2 darts in one event) correctly, which the sum saw as a takeout.
- Also works for darts that were already stuck before the start (without this, pulling old darts counted as a throw, happened in a real game).
- Without empty images fallback: change predominantly where darts detected since the last takeout are stuck (then start with an empty board).

**Locking the exposure:** `vision.OpenCameras` and `vision.OpenCamera` wait 1 s after opening (measured: settled after ~0.5 s) and then locks exposure and white balance (`camera.Camera.LockExposure`). Only works on macOS; why the camera resets it on open/close and why Linux and Windows can't: [`pkg/camera` → exposure](../../pkg/camera/AGENTS.md#exposure). Whether the lock really holds during the takeout hasn't been checked with a real throw yet.

## Finding cameras

The device order changes constantly under macOS (observed with ffmpeg: FaceTime camera sometimes index 0, sometimes 2, sometimes 3), and all board cameras have the same name. Therefore the calibration doesn't contain device IDs but per camera its `device_name` (and `device_model`, only shown) and a reference image (`reference`, taken when the camera is added and again when it is calibrated). `vision.OpenCameras` groups the cameras by device name, opens all devices with that name and assigns them via image comparison (`AssignCameras`, 64×36 grayscale thumbnail; measured: same camera 6–8, other board camera 47–61, FaceTime 67–69, threshold 25). So cameras of different models can be mixed. A missing camera only affects itself (`vision.ErrNotConnected`, the others are assigned anyway); if the views don't match (`ErrAmbiguousCameras`), the cameras of that name are opened in device order to calibrate them again, see [App](#app). (Fixed device IDs per camera existed until 2026-10-13 and were dropped: they change when replugging.)

**Stable identifier:** the three Autodarts cameras have the same VID:PID (1e45:8022) *and* the same serial number (200910120001); macOS unique IDs and Linux `/dev/v4l/by-path` depend on the USB port. So there is no hardware ID per camera – name (plus model) selects the group, the image the individual camera. This survives replugging into other ports.

## Accuracy

- With 8 real reference darts (two rounds, tips located by hand, fields confirmed by the player) 7 are correct; the eighth lies 1 mm next to the wire on the wrong side (`TestRealDarts`). Individual cameras are off more often (farther, compressed image edge; black dart in front of a black segment invisible), the median catches that.
- First real throws (4, recorded with `-save`): all fields correct, 11 of 12 camera images deliver a tip, the cameras are 2–16 mm apart per throw; two of them are regression tests (`TestRealThrows`, `testdata/throws`).
- Second round (stuck by hand, recorded during the takeout – before/after swapped doesn't matter for the comparison): D19 and outer bull correct; T20 as S20, because the dart sits at the inner triple wire and all three cameras consistently measure 1–2 mm inside; bullseye not detected, because the neighboring dart was moved along during the takeout (two changes at once).
- With fine calibration: darts not directly at the wire are reliably scored correctly; darts < 1.5 mm from the wire go either way (fine calibration fixes S5/S12 and brings T20 to 0.2 mm from the border, but S3/S17 flips instead) – there the tip detection rather than the calibration is the bottleneck. `TestRealDarts` therefore doesn't check such darts, it logs them.
- Another case (2026-10-11): T4 at the inner triple wire as S4 (cameras 99.2/99.0/96.2 mm, median 97.6). Tried and rejected: pushing the tip along the axis to the end (instead of the center of the last 3 px), weaker blur (±0.5 mm), intersection of the dart axis lines instead of median (T4 only up to 98.3 mm, one bad measurement pulled the bullseye 90 mm away).

## Debugging

- `darts-o-cam -v` reports every motion, `-save <dir>` saves the images of all cameras before/after at every event (named after the event's number in the web interface); `darts-o-cam analyze -dir <dir>` explains the detection of such saved events with the cameras of the configuration.
- Caution: images can show people (e.g. wrongly assigned laptop camera), don't put them into the repo.

## Known limitations

- No handling of bounce-outs, tips hidden by darts already stuck, darts that stay stuck during the takeout.
- A camera very close to the flight (2026-10-13, cam3, event 30): flight and barrel are one piece of 15,637 px whose lower half (cut by image height, the dart lying diagonally) still holds part of the flight; its elongation is 4.0, just at the limit (`MinElongation` 4), so no tip, although the dart is clearly visible. Real throws' lower halves are 7–9; with `MinElongation` 3.5 that camera found the tip correctly (3 mm from the others). Not changed by default yet: one case, and the risk of other changes passing as darts is unmeasured; the lower third doesn't help (4.1, other throws get less elongated). Better would be cutting along the dart's own axis or separating the flight by its width.
- Tip overshoots: with the T13 (2026-10-11, `/tmp/darts-debug` event 010) the changed area at cam1 extended ~11 px beyond the visible tip (dark spot/shadow at the entry point, plus blur), 1.8 mm too far inside. Second case right afterwards (T13 as S13, event 018: cam1 96.8, cam2 97.7, cam3 101.1 mm): again ~2 mm inwards, the calibration was only ≤ 0.4 mm off on the same images (`RefineCalibration` on the before image). In the unsharpened image the tip is dark red up to the wire (not gray like steel, rather shadow or reflection of the red ring), the change really reaches there. Tried and rejected: pulling the tip back in the unsharpened image by at most the blur radius – it moved by ≤ 1 px, no field changed. A correction needs a measurement series with known positions, taken in darts-o-cam itself (planned: a guided routine "stick a dart into T20, …", see [TODO.md](../../TODO.md)); darts-o-cam has to send correct positions, the server doesn't correct anything (user's decision, 2026-10-13).

## Hardware

Development computer, as of 2026-10-11: board with three cameras ("Autodarts DIY Cam"), connected via a USB-C adapter to a MacBook Pro (Intel, macOS 26):

- The three cameras hang on a USB 2.0 hub on the board (`0x214b:0x7250`), all with `0x1e45:0x8022` and **the same serial number** `200910120001`. They can only be distinguished via the USB port (macOS location ID `0x14111200`, `0x14111300`, `0x14111400`; camera unique ID = location ID + vendor + product, e.g. `0x141112001e458022`). The mapping only holds as long as board and adapter are plugged the same way.
- UVC, under macOS via AVFoundation (`uniqueID` = location ID + VID + PID, e.g. `0x141112001e458022`; the FaceTime camera is `0x8020000005ac8514`). Modes up to 1280×720 at 30 fps; all three deliver full 30 fps in 720p simultaneously (compressed over USB 2.0). Several programs can open the same camera at the same time under macOS (tested: ffmpeg `detect` and `pkg/camera` in parallel).
- The cameras sit around the light ring and look obliquely at the board from different sides. The device order isn't stable, see [finding cameras](#finding-cameras).
- The same adapter also holds a VIA hub, card reader and Realtek LAN; they don't belong to the board.

Measurements of camera access per operating system on this board (and a Linux/Windows laptop with i5-11400H): [`pkg/camera`](../../pkg/camera/AGENTS.md).

## Further options

If the current approach isn't enough (check how well the libraries are maintained before deciding):

- **GoCV/OpenCV:** if an OpenCV package is available on the target platform (e.g. Linux/Raspberry Pi), for more robust building blocks (contours, line fitting, camera calibration against lens distortion).
- **Neural network via ONNX Runtime** (e.g. [onnxruntime_go](https://github.com/yalue/onnxruntime_go), also cgo): a trained model detects the dart tips directly, more robust with light and occlusion, but needs training data and a model.
  Only tackle this if the classical approach isn't reliable enough.
- **Python with OpenCV as a separate service** that reports throws via NATS/HTTP: only as a last resort, because it leaves the Go stack.
