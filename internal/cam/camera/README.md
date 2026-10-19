# internal/cam/camera

A Go package that reads images from the cameras connected to the computer – directly from the operating system, without ffmpeg or other programs, and without cgo. [darts-o-cam](../../../cmd/darts-o-cam/README.md) uses it to watch the board. Part of [darts-o-mat](../../../README.md).

## Supported systems

| System | Via | Notes |
| --- | --- | --- |
| macOS | AVFoundation | Asks for camera permission for the terminal on first use. Several programs can use a camera at once. |
| Linux | V4L2 | Your user needs access to `/dev/video*` (usually the group `video`). Uses libjpeg-turbo if installed, which needs far less CPU. |
| Windows | Media Foundation | Allow camera access for desktop apps in the privacy settings. Only one program can use a camera at a time. |

Other systems return `camera.ErrUnsupported`. Cameras have the same name on all three systems, so a calibration made on one works on the others.

## Usage

```go
devices, err := camera.Devices()
// …
cam, err := camera.Open(devices[0].ID, camera.Options{Width: 1280, Height: 720, FPS: 10})
if err != nil {
	log.Fatal(err)
}
defer cam.Close()

img, err := cam.Read() // waits for the next frame, an *image.RGBA
```

`LockExposure` fixes exposure and white balance so that the images don't change when a hand moves in front of the camera. This only works on macOS; on Linux and Windows it returns `camera.ErrExposureLockUnsupported`.

## Linux: libjpeg-turbo

If `libturbojpeg.so.0` is installed (Debian/Ubuntu/Raspberry Pi OS: `libturbojpeg0`, Arch: `libjpeg-turbo`), it is loaded at runtime; otherwise Go's own JPEG decoder is used. `cam.Format` tells which one is running.

- On NixOS the library isn't in the search path: point `LD_LIBRARY_PATH` at the (64-bit) `libjpeg_turbo` package.
- Loading it makes the binary depend on glibc, even with `CGO_ENABLED=0`. For a fully static binary (e.g. Alpine), build with `-tags noturbojpeg`.

Implementation details and measurements: [AGENTS.md](AGENTS.md).
