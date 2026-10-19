// Package camapp is darts-o-cam at runtime: the board's cameras, their calibration and
// settings (kept on this computer, never on the server), and the detection that reports
// darts to darts-o-mat. internal/cam/camui is its web interface.
package camapp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/sj14/darts-o-mat/internal/cam/camera"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

// configFile is the name of the configuration in the config directory; the cameras'
// images are stored next to it.
const configFile = "config.json"

// Config is everything darts-o-cam keeps on this computer.
type Config struct {
	// Boards are the boards this computer has cameras at; there is always at least one.
	Boards []BoardConfig `json:"boards"`
}

// Board is what is set of a board in the web interface.
type Board struct {
	// ID tells the board apart for good, its name may change.
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Server and Token connect to darts-o-mat (the token of the board, created on the
	// server's boards page). Without them, darts are only shown.
	Server string `json:"server,omitempty"`
	Token  string `json:"token,omitempty"`
	// Width and Height are the wanted frame size: each camera uses its own mode closest to
	// it (see camera.Options). FPS limits the frames analyzed per second.
	Width  int `json:"width"`
	Height int `json:"height"`
	FPS    int `json:"fps"`
	// Detection are the thresholds of the detection, in board millimetres.
	Detection vision.WatchOptions `json:"detection"`
}

// BoardConfig is a board with its cameras.
type BoardConfig struct {
	Board
	// Cameras are the board's cameras with their calibration, see vision.Calibration.
	// Their names are unique across all boards, they name the cameras' images.
	Cameras []vision.CameraCalibration `json:"cameras"`
	// Detect is whether the detection runs; it is kept for the next start.
	Detect bool `json:"detect"`
}

// newBoard returns a board without cameras and with the default settings.
func newBoard(id int, name string) BoardConfig {
	return BoardConfig{
		Board:   Board{ID: id, Name: name, Width: 1280, Height: 720, FPS: 10, Detection: vision.DefaultWatchOptions},
		Cameras: []vision.CameraCalibration{},
		Detect:  true,
	}
}

// defaultConfig returns the configuration without a file: one board without cameras.
func defaultConfig() Config {
	return Config{Boards: []BoardConfig{newBoard(1, "Board")}}
}

// options are the camera settings of the board.
func (b Board) options() camera.Options {
	return camera.Options{Width: b.Width, Height: b.Height, FPS: b.FPS}
}

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
	cfg := defaultConfig()
	data, err := os.ReadFile(filepath.Join(dir, configFile))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err = json.Unmarshal(data, &cfg); err == nil && len(cfg.Boards) == 0 {
		cfg.Boards = defaultConfig().Boards
	}
	return cfg, err
}

// cameras returns the cameras of all boards.
func (c Config) cameras() []vision.CameraCalibration {
	var all []vision.CameraCalibration
	for _, b := range c.Boards {
		all = append(all, b.Cameras...)
	}
	return all
}

// board returns the board with the given ID, nil if there is none.
func (c *Config) board(id int) *BoardConfig {
	for i := range c.Boards {
		if c.Boards[i].ID == id {
			return &c.Boards[i]
		}
	}
	return nil
}

// camera returns the board of the camera named name and its index there, nil if there
// is no such camera.
func (c *Config) camera(name string) (*BoardConfig, int) {
	for i := range c.Boards {
		b := &c.Boards[i]
		if j := slices.IndexFunc(b.Cameras, func(cc vision.CameraCalibration) bool { return cc.Name == name }); j >= 0 {
			return b, j
		}
	}
	return nil, -1
}

// Calibration returns the cameras of all boards as a vision.Calibration with their images
// in dir.
func (c Config) Calibration(dir string) vision.Calibration {
	return vision.NewCalibration(dir, c.cameras())
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
