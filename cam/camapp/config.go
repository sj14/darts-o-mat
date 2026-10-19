// Package camapp is darts-o-cam at runtime: the board's cameras, their calibration and
// settings (kept on this computer, never on the server), and the detection that reports
// darts to darts-o-mat. cam/camui is its web interface.
package camapp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/sj14/darts-o-mat/cam/vision"
)

// configFile is the name of the configuration in the config directory; the cameras'
// images are stored next to it.
const configFile = "config.json"

// Config is everything darts-o-cam keeps on this computer.
type Config struct {
	// Cameras are the board's cameras with their calibration, see vision.Calibration.
	Cameras []vision.CameraCalibration `json:"cameras"`
	// Server and Token connect to darts-o-mat (the token of the board, created on the
	// server's boards page). Without them, darts are only shown.
	Server string `json:"server,omitempty"`
	Token  string `json:"token,omitempty"`
	// Detect is whether the detection runs; it is kept for the next start.
	Detect bool `json:"detect"`
	// Width and Height are the wanted frame size: each camera uses its own mode closest to
	// it (see camera.Options). FPS limits the frames analyzed per second.
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
	// Detection are the thresholds of the detection, in board millimetres.
	Detection vision.WatchOptions `json:"detection"`
}

var defaultConfig = Config{Cameras: []vision.CameraCalibration{}, Detect: true, Width: 1280, Height: 720, FPS: 10, Detection: vision.DefaultWatchOptions}

// DefaultDir is where darts-o-cam keeps its configuration, e.g. ~/.config/darts-o-cam on
// Linux, ~/Library/Application Support/darts-o-cam on macOS, %AppData%\darts-o-cam on Windows.
func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "darts-o-cam"), nil
}

// LoadConfig reads the configuration in dir; without one, it is the default.
func LoadConfig(dir string) (Config, error) {
	cfg := defaultConfig
	data, err := os.ReadFile(filepath.Join(dir, configFile))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// Calibration returns the cameras as a vision.Calibration with their images in dir.
func (c Config) Calibration(dir string) vision.Calibration {
	return vision.NewCalibration(dir, c.Cameras)
}

// save writes the configuration to dir. Only the user may read it: it holds the token.
// It is written to a temporary file first, so a crash doesn't leave half a file.
func (c Config) save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, configFile+".tmp")
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, configFile))
}
