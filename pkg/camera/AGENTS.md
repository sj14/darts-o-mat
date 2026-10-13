# pkg/camera

Camera access directly from the operating system, without external programs: macOS (AVFoundation via [purego](https://github.com/ebitengine/purego)), Linux (V4L2 via ioctl with `golang.org/x/sys/unix`) and Windows (Media Foundation via COM with `golang.org/x/sys/windows`); other systems return `camera.ErrUnsupported`.
Decided with the user on 2026-10-11; **macOS, Linux and Windows implemented**.
Used by [darts-o-cam](../../cmd/darts-o-cam/AGENTS.md) (via `pkg/vision`); project-wide rules: [root AGENTS.md](../../AGENTS.md). For humans: [README.md](README.md). Open tasks: [TODO.md](TODO.md). The development board's hardware: [darts-o-cam → hardware](../../cmd/darts-o-cam/AGENTS.md#hardware).

## Goal and decisions

- **Goal:** darts-o-cam without external programs on Linux, macOS and Windows. Without cgo is desired, but not a must (see [root AGENTS.md → technologies](../../AGENTS.md#technologies)).
- **No ffmpeg fallback** (user's decision): ffmpeg is out completely.
- GoCV was rejected, see [root AGENTS.md → technologies](../../AGENTS.md#technologies).

## Interface

One backend per operating system (`camera_darwin.go`, `camera_linux.go`, `camera_windows.go`, otherwise `camera_other.go` with `ErrUnsupported`) and a small interface:

- `Devices()` (ID, name, model)
- `Open(id, Options{Width, Height, FPS})`
- `Read()` waits for a new frame (frames in between are dropped, the frame belongs to the caller)
- `LockExposure()`
- `Close()` (may be called more than once)

The shared part (`camera.go`) is tested without a camera: handing over the frames (`TestRead`), the fps limit (`TestDue`), the watchdog (`TestWatchdog`) and the BGRA → RGBA swap of macOS and Windows (`TestSwapBGRA`). Each backend only calls it:

- `newCamera(device, opts)` when opening, `watch()` once the camera runs.
- For every frame from the camera, on the one goroutine (or serial queue) delivering frames: `due()` before copying or decoding it – it implements the fps limit (with some jitter allowed, so a 30 fps camera delivers every third frame at 10 fps; a broken frame uses up its slot) – then `deliver(img)`, or `fail(err)` when the camera stops working.
- The watchdog (`watch`, a `time.AfterFunc` reset by `deliver`) fails the camera when it delivers no frame for 5 s (unplugged); this used to be done separately by each backend (unified 2026-10-12).
- `Close` calls the backend's `stop`.

 `pkg/vision` (`OpenCameras`, `OpenCamera`, `Live`) and `pkg/camapp` (device list) only use this interface.

Names are the same on all systems (macOS name, Linux USB product from sysfs, Windows friendly name), so calibrations work across systems. `Device.ID` depends on the USB port on all systems and changes when replugging; that's why darts-o-cam assigns the cameras by image, see [darts-o-cam → finding cameras](../../cmd/darts-o-cam/AGENTS.md#finding-cameras).

## macOS

AVFoundation via purego (Apache 2.0, approved by the user) instead of cgo – no Xcode needed to build, `CGO_ENABLED=0` works.

- Frameworks via `Dlopen`, C functions of CoreMedia/CoreVideo/libdispatch via `RegisterLibFunc`, constants (`AVMediaTypeVideo`, `kCVPixelBufferPixelFormatTypeKey`, session presets) via `Dlsym`.
- A delegate class registered at runtime (`DartsOCamFrameDelegate`) receives the frames on a serial dispatch queue per camera; format BGRA (swapped to RGBA in Go); the size is the session preset the camera can do (`canSetSessionPreset:`, only meaningful once the input is added) closest to the wanted size, frames aren't scaled (until 2026-10-13 they were scaled to the wanted size via `videoSettings`, even up; now all three systems behave the same: the camera's own mode closest to the wanted size, see `camera.Options`); the fps limit already drops frames in the callback (before copying).
- No ARC: alloc/init objects are freed with `release`, otherwise an autorelease pool per call on a fixed thread (`withPool`). On close the queue is drained with `dispatch_sync` before the delegate is released.
- `Device.ID` is the `uniqueID` (location ID + VID + PID), `Device.Model` VID:PID from `AVCaptureDevice.modelID`.
- Camera permission: requested the first time (dialog for the terminal), otherwise an error pointing to the system settings.
- Several programs can use the same camera at the same time.
- Measured on the development board: 3 cameras 1280×720 at 10 fps with ~20 % CPU, ~130 MB.
- **Load** (measured 2026-10-13 with `sample` and `ps`, darts-o-cam with detection running, Intel i7-9750H): darts-o-cam 22 % of a core, of which ~7–8 % the frame callback (`crosscall2`: copy and BGRA → RGBA swap, plus allocating the frame); the rest is spread over the detection and the runtime. Larger are macOS' own processes: `VTDecoderXPCService` 48 % (decodes the cameras' MJPEG) and `UVCAssistant` 19 % (USB camera driver). They handle all 30 frames per second of every camera: the fps limit only drops frames once decoded, and the Autodarts cameras offer **only 30 fps** in every size (`videoSupportedFrameRateRanges` 30–30, 160×120 to 1280×720; the FaceTime camera 1–30), so the rate can't be lowered at the source (setting `activeVideoMinFrameDuration` would only work for cameras that offer lower rates). A smaller size would decode less, but makes the detection coarser – not without measuring it.
- **BGRA → RGBA** (`swapBGRA`, shared with Windows) works on whole pixels as uint32 via `unsafe.Slice`: 1.45 instead of 2.4 ms per 1280×720 frame. Pairs of pixels as uint64 took 1.25 ms, but need rows aligned to 8 bytes (an odd width isn't; Go's checkptr in race mode rejects that); the experimental `simd` package (Go 1.27, `GOEXPERIMENT=simd`, vectors of 4 uint32 on this CPU, And/Or/shifts – no byte shuffle in the portable API) 1.06 ms: copying 7.4 MB per frame is the limit, not computing. Not worth an experiment.

## Linux

Implemented 2026-10-12. V4L2 via ioctl with `golang.org/x/sys/unix` (structs from `videodev2.h` rebuilt, sizes via `unsafe.Sizeof`, so that 32 and 64 bit are right), 4 driver buffers mapped via `mmap`.

- Format MJPEG (necessary with 3 cameras on USB 2.0), decoded with `image/jpeg`; if the camera can't do MJPEG, YUYV. The driver chooses the resolution (closest mode to the requested one, no scaling like under macOS).
- One goroutine per camera waits for frames with `poll`, an `eventfd` stops it on close. The fps limit drops frames before copying and decoding; broken MJPEG frames are skipped.
- Devices: `/dev/video*`, only those with video capture (UVC cameras have a second device for metadata). `Device.ID` is the device path (`/dev/video2`, changes when replugging and rebooting; `/dev/v4l/by-path` has two links `usb`/`usbv2` per camera, therefore not used). Name and model come from the USB device in sysfs (`product`, `idVendor:idProduct`), because V4L2 truncates the card name to 31 characters ("Autodarts DIY Cam: Autodarts DI").
- Measured on the development board (Linux, i5-11400H): 3 cameras 1280×720 at 10 fps, ~100 MB; assignment via reference image with the macOS reference images correct.

**libjpeg-turbo optional** (user's wish 2026-10-12, `turbojpeg_linux.go`): if `libturbojpeg.so.0` is installed, purego loads it at runtime (TurboJPEG 2 API, also available in libjpeg-turbo 3) and decodes directly to RGBA, one handle per camera; otherwise `image/jpeg`.

- Per frame 3.5 instead of 18 ms (`image/jpeg` 11.5 ms + 5.6 ms converting to RGBA), the detection 15 % instead of 56 % of a core.
- Frames with a warning (e.g. extraneous bytes) count, only fatal errors drop the frame. `Camera.Format` says which decoder runs; darts-o-cam logs it when it opens a camera.
- **Caution:** because of purego, even a build with `CGO_ENABLED=0` depends on the glibc loader (previously static) – doesn't run under musl/Alpine and under NixOS only with nix-ld. With the build tag `noturbojpeg` libjpeg-turbo is left out (`turbojpeg_none_linux.go`) and the binary is static again. Under NixOS the library isn't in the search path: set `LD_LIBRARY_PATH` to `libjpeg_turbo` (64-bit variant).
- Rejected: VA-API (own parsing of the JPEG header, driver-dependent, lots of code) and OpenGL (can't decode JPEG, only the conversion, reading back eats the gain).

**Open (with the user 2026-10-12): Go package `simd`.** (For the BGRA swap on macOS/Windows it was measured on 2026-10-13 and isn't worth it, see macOS above.) Go 1.27 has an architecture-independent `simd` (hardware on amd64/arm64/wasm, otherwise emulated in Go), but only with `GOEXPERIMENT=simd`, without compatibility promise and with known bugs – therefore not used yet. Profile of `image/jpeg` + conversion (720p): ~30 % `DrawYCbCr` (YCbCr → RGBA), ~40 % IDCT/`reconstructBlock`, ~25 % Huffman (sequential, not via SIMD). As soon as `simd` is no longer experimental: write the YCbCr → RGBA conversion for the case without libjpeg-turbo with `simd` (little code, estimated 18 → ~13 ms per frame, measure first). The IDCT would need a copy of `image/jpeg` (~2000 lines, functions not exported) and would only get to an estimated 7–8 ms – don't do it, libjpeg-turbo remains the way to low CPU load.

## Windows

Implemented 2026-10-12, `camera_windows.go`. Media Foundation via `syscall.SyscallN` through the vtables of the COM objects (`comObject.call` with method index, `//go:uintptrescapes`, so that pointers to Go variables stay valid during the call), DLLs with `golang.org/x/sys/windows`; no cgo, no further library. COM in the multithreaded apartment on a fixed thread (`withCOM`), `MFStartup` once.

- Devices via `MFEnumDeviceSources`; `Device.ID` is the symbolic link (`\\?\usb#vid_1e45&pid_8022&mi_00#7&3549ee4f&0&0000#{…}\global`, contains the USB port, changes when replugging), model from `vid_`/`pid_` in it, name = friendly name.
- A synchronous source reader with `MF_SOURCE_READER_ENABLE_ADVANCED_VIDEO_PROCESSING`: from the native modes the one with the closest size is chosen, then MJPEG, then the most fps; output RGB32 (BGRX, stride from `MF_MT_DEFAULT_STRIDE`, converted to RGBA in Go). So Media Foundation decodes, all 30 frames per second; the fps limit then drops before copying. The buffers have no `IMF2DBuffer`, therefore `IMFMediaBuffer::Lock`.
- One goroutine per camera waits in `ReadSample`; this **can't be interrupted** (tested: neither `Shutdown` nor `Stop` nor `Flush` of the source reliably end a waiting `ReadSample`, `Stop` hung in one of three cases), therefore `Close` sets a flag and the loop ends with the next frame; if the camera delivers no more frames, `Close` gives up after 5 s with an error (reader stays open).
- Unplugging (tested on the development board): `ReadSample` returns immediately with `MF_E_VIDEO_RECORDING_DEVICE_INVALIDATED` (`Read`: "camera is gone (unplugged?)"), for one camera 2.5 s after the last frame, so before the watchdog; `Close` afterwards in ~3 ms.
- Unlike under macOS only one program can use a camera (second one: `MF_E_HW_MFT_FAILED_START_STREAMING`, reported as "used by another program"), e.g. not two darts-o-cam at the same time; darts-o-cam itself shares its open cameras between detection and web interface, and takes the still images of devices it doesn't use by opening them for a moment.
- Measured on the development board (i5-11400H): 3 cameras 1280×720, ~100 MB; assignment via reference image with the macOS reference images correct.

**Load, because all 30 frames are decoded** (investigated 2026-10-12, only the camera part without detection, median of 7 alternating runs; the laptop's clocking makes individual measurements scatter strongly, so always measure alternating and several times):

- The cameras offer exactly 30 fps in every size under Windows (`MF_MT_FRAME_RATE_RANGE_MIN` = MAX = 30), requesting fewer isn't possible.
- Expensive isn't the MJPEG decoding (only decoding to YUY2: ~6 %), but the conversion to RGB32: with `MF_SOURCE_READER_ENABLE_VIDEO_PROCESSING` 17 %, with the advanced converter **8 %** (therefore chosen, one constant).
- Rejected: YUY2 from the reader and converting only the 10 kept frames in Go (`yuyvToRGBA` from Linux, 6.7 ms per frame) – 7 %, so hardly better, more code, and red turns pink, because `color.YCbCrToRGB` assumes full JPEG range, but Windows delivers limited video range.
- Also not worth it: taking undecoded MJPEG and decoding only every third frame – with `image/jpeg` (18 ms per frame) more expensive than now, with Media Foundation's MJPEG decoder by hand (MFT with `ProcessInput`/`ProcessOutput`, own output buffers, color conversion afterwards) lots of COM code for a few percent.

## Exposure

- `LockExposure` locks exposure and white balance; the mode is read back, because UVC cameras could also ignore it. When to call it: [darts-o-cam → flow](../../cmd/darts-o-cam/AGENTS.md#flow).
- **Caution:** UVC cameras keep the lock in hardware, also for other programs and the next start – in the test the images were much darker afterwards and no longer matched the reference images (distance 62 instead of < 25). Therefore `Open` and `Close` always reset the camera to automatic.
- **Locking doesn't work under Linux** (`LockExposure` returns `ErrExposureLockUnsupported`): UVC cameras don't report the chosen exposure time in automatic mode, V4L2 returns the default value (development board: 166, flag `INACTIVE`; the automatic corresponded to ~290, there is no gain control). Switched to manual, the images became 40–60 % darker as a result. Possible would be readjusting the exposure time after switching until the brightness matches again (roughly linear: 166 → 54, 300 → 92 at automatic 87) – but costs seconds per camera and code; not decided yet. `Open` still sets the camera to automatic (aperture priority, otherwise auto), in case macOS left a lock in the hardware.
- **Locking doesn't work under Windows** either (`ErrExposureLockUnsupported`): `IAMCameraControl::Get` returns the default value (-6 = 1/64 s) or the last manual value in automatic mode, not the automatic's one; white balance likewise (4600 K). Locked, the images became 3× darker or 2× brighter (mean brightness 95 → 30, 86 → 199). Besides, Windows only knows the exposure in powers of two. `Open`/`Close` set exposure and white balance to automatic (`IAMCameraControl`/`IAMVideoProcAmp`), in case a lock is stuck in the hardware – in the test that was actually the case after an aborted run.

## Libraries checked

- [gen2brain/cam2ip](https://github.com/gen2brain/cam2ip) (package `camera`): without cgo on all three systems. Tested on the development board: 3 cameras simultaneously with 1280×720 at ~29.5 fps. Rejected because of GPL-3.0; besides, `Read()` only returns the last frame in the same buffer without waiting for a new one, and provides no model. Don't take over the code (GPL), write the own backend following Apple's documentation.
- [svanichkin/gocam](https://github.com/svanichkin/gocam) (MIT): unusable. It only opens the default camera or `/dev/video0` with global state (no three cameras), fixed 352×288 under macOS, no device list, cgo.
- Pure Linux packages (go4vl, ch3ri0ur/go-v4l2, korandiz/v4l) only cover Linux.
