package vision

import (
	"fmt"
	"image"
	"io"
	"os/exec"
	"runtime"
	"strconv"
)

// Camera reads frames from a camera through ffmpeg, which works the same on macOS
// (AVFoundation) and Linux (V4L2) without cgo.
type Camera struct {
	Device string

	cmd    *exec.Cmd
	out    io.ReadCloser
	width  int
	height int
}

// OpenCamera starts reading the camera at the given resolution and frame rate.
// device is the AVFoundation index on macOS (e.g. "1") or the device path on Linux (e.g. "/dev/video0").
func OpenCamera(device string, width, height, fps int) (*Camera, error) {
	size := fmt.Sprintf("%dx%d", width, height)
	var input []string
	switch runtime.GOOS {
	case "darwin":
		// AVFoundation only accepts the frame rates the camera offers, so the rate is reduced by the filter below.
		input = []string{"-f", "avfoundation", "-framerate", "30", "-video_size", size, "-pixel_format", "nv12", "-i", device}
	default:
		input = []string{"-f", "v4l2", "-input_format", "mjpeg", "-framerate", "30", "-video_size", size, "-i", device}
	}
	args := append([]string{"-hide_banner", "-loglevel", "error"}, input...)
	args = append(args, "-vf", fmt.Sprintf("fps=%d", fps), "-f", "rawvideo", "-pix_fmt", "rgba", "-")

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}
	return &Camera{Device: device, cmd: cmd, out: out, width: width, height: height}, nil
}

// Read blocks until the next frame arrives.
func (c *Camera) Read() (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, c.width, c.height))
	if _, err := io.ReadFull(c.out, img.Pix); err != nil {
		return nil, err
	}
	return img, nil
}

func (c *Camera) Close() error {
	_ = c.cmd.Process.Kill()
	_ = c.cmd.Wait()
	return nil
}

// OpenCameras opens the cameras of a calibration, in the calibration's order.
// Cameras without a fixed device are found among the devices named DeviceName
// by comparing their view with the reference images (macOS only).
func OpenCameras(calib Calibration, width, height, fps int) ([]*Camera, error) {
	cams := make([]*Camera, len(calib.Cameras))
	closeAll := func() {
		for _, c := range cams {
			if c != nil {
				c.Close()
			}
		}
	}

	var unresolved []int
	for i, c := range calib.Cameras {
		if c.Device == "" {
			unresolved = append(unresolved, i)
			continue
		}
		cam, err := OpenCamera(c.Device, width, height, fps)
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("camera %s: %w", c.Name, err)
		}
		cams[i] = cam
	}
	if len(unresolved) == 0 {
		return cams, nil
	}

	if runtime.GOOS != "darwin" {
		closeAll()
		return nil, fmt.Errorf("camera %s: set its device", calib.Cameras[unresolved[0]].Name)
	}
	devices, err := ListAVFoundationDevices()
	if err != nil {
		closeAll()
		return nil, err
	}

	var candidates []*Camera
	var frames []image.Image
	closeCandidates := func(keep map[int]bool) {
		for i, c := range candidates {
			if !keep[i] {
				c.Close()
			}
		}
	}
	for _, d := range devices {
		if d.Name != calib.DeviceName {
			continue
		}
		cam, err := OpenCamera(strconv.Itoa(d.Index), width, height, fps)
		if err != nil {
			closeCandidates(nil)
			closeAll()
			return nil, err
		}
		candidates = append(candidates, cam)
	}
	// Let the exposure settle for about a second before comparing views.
	for _, cam := range candidates {
		var img *image.RGBA
		for range max(fps, 1) {
			if img, err = cam.Read(); err != nil {
				closeCandidates(nil)
				closeAll()
				return nil, err
			}
		}
		frames = append(frames, img)
	}

	var refs []image.Image
	for _, i := range unresolved {
		ref, err := calib.LoadReference(calib.Cameras[i])
		if err != nil {
			closeCandidates(nil)
			closeAll()
			return nil, err
		}
		refs = append(refs, ref)
	}
	assignment, err := AssignCameras(frames, refs)
	if err != nil {
		closeCandidates(nil)
		closeAll()
		return nil, fmt.Errorf("%d devices named %q: %w", len(candidates), calib.DeviceName, err)
	}

	keep := map[int]bool{}
	for r, f := range assignment {
		cams[unresolved[r]] = candidates[f]
		keep[f] = true
	}
	closeCandidates(keep)
	return cams, nil
}
