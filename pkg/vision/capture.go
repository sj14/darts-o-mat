package vision

import (
	"fmt"
	"image"
	"io"
	"os/exec"
	"runtime"
)

// Camera reads frames from a camera through ffmpeg, which works the same on macOS
// (AVFoundation) and Linux (V4L2) without cgo.
type Camera struct {
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
	return &Camera{cmd: cmd, out: out, width: width, height: height}, nil
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
