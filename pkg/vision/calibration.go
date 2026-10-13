package vision

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// Ring names a circle of the board that calibration points can lie on.
type Ring string

const (
	RingDoubleOuter Ring = "double_outer"
	RingDoubleInner Ring = "double_inner"
	RingTripleOuter Ring = "triple_outer"
	RingTripleInner Ring = "triple_inner"
)

var ringRadius = map[Ring]float64{
	RingDoubleOuter: dartboard.DoubleOuterRadius,
	RingDoubleInner: dartboard.DoubleInnerRadius,
	RingTripleOuter: dartboard.TripleOuterRadius,
	RingTripleInner: dartboard.TripleInnerRadius,
}

// CalibrationPoint is a point found in the camera image whose position on the board is known:
// where a ring crosses the wire between two neighboring numbers, e.g. 20 and 1.
type CalibrationPoint struct {
	Image   dartboard.Point `json:"image"`   // pixels
	Between [2]uint8        `json:"between"` // the neighboring numbers, clockwise
	Ring    Ring            `json:"ring"`
}

// Board returns the position of the point on the board.
func (c CalibrationPoint) Board() (dartboard.Point, error) {
	r, ok := ringRadius[c.Ring]
	if !ok {
		return dartboard.Point{}, fmt.Errorf("unknown ring %q", c.Ring)
	}
	a, b := dartboard.SegmentIndex(c.Between[0]), dartboard.SegmentIndex(c.Between[1])
	if a < 0 || b < 0 || (a+1)%len(dartboard.Segments) != b {
		return dartboard.Point{}, fmt.Errorf("%d and %d are not clockwise neighbors", c.Between[0], c.Between[1])
	}
	return dartboard.Polar(r, (float64(a)+0.5)*dartboard.SegmentAngle), nil
}

type CameraCalibration struct {
	Name string `json:"name"`
	// DeviceName is the name of the camera's capture device, e.g. "Autodarts DIY Cam".
	// It selects the camera independent of the USB port; among cameras with the same name,
	// which one is which is told by their view (see Reference and AssignCameras).
	DeviceName string `json:"device_name"`
	// DeviceModel is the USB vendor:product id of the device, e.g. "1e45:8022", to
	// recognize it when shown; it is not used to select it.
	DeviceModel string `json:"device_model,omitempty"`
	// Reference is an image of the camera's view, relative to the calibration file.
	Reference string `json:"reference,omitempty"`
	// Width and Height are the size of the frames the camera was calibrated on: the
	// calibration is in pixels and doesn't fit frames of another size.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Empty is an image of the empty board, relative to the calibration file. It lets the
	// detection tell darts pulled out from darts thrown (see Watcher.Empty).
	Empty string `json:"empty,omitempty"`
	// Points are the rough calibration, e.g. clicked by hand.
	Points []CalibrationPoint `json:"points"`
	// Refined is the calibration improved by RefineCalibration (image pixels to board mm).
	// If set, it is used instead of the points.
	Refined *Homography `json:"refined,omitempty"`
	// Correction moves the refined homography's grid onto the rings found by
	// RefineCalibration, see GridCorrection.
	Correction *GridCorrection `json:"correction,omitempty"`
}

// Calibrated reports whether the camera has a calibration for frames of the given size.
func (c CameraCalibration) Calibrated(width, height int) bool {
	_, err := c.ToBoard()
	return err == nil && c.Width == width && c.Height == height
}

// ToBoard returns the homography mapping image pixels to board millimeters:
// the refined one if there is one, otherwise the one from the points.
func (c CameraCalibration) ToBoard() (Homography, error) {
	if c.Refined != nil {
		return *c.Refined, nil
	}
	return c.FromPoints()
}

// Mapping returns how the camera's image maps to the board: ToBoard with the correction.
func (c CameraCalibration) Mapping() (Mapping, error) {
	h, err := c.ToBoard()
	if err != nil {
		return Mapping{}, err
	}
	m := Mapping{ToBoard: h}
	if c.Refined != nil {
		m.Correction = c.Correction
	}
	return m, nil
}

// FromPoints returns the homography from the calibration points only.
func (c CameraCalibration) FromPoints() (Homography, error) {
	var image, board []dartboard.Point
	for _, p := range c.Points {
		b, err := p.Board()
		if err != nil {
			return Homography{}, fmt.Errorf("camera %s: %w", c.Name, err)
		}
		image = append(image, p.Image)
		board = append(board, b)
	}
	return EstimateHomography(image, board)
}

// Calibration are the cameras of a board.
type Calibration struct {
	Cameras []CameraCalibration `json:"cameras"`

	dir string // of the calibration file, for the reference images
}

// NewCalibration returns a calibration of the cameras whose images are in dir.
func NewCalibration(dir string, cameras []CameraCalibration) Calibration {
	return Calibration{Cameras: cameras, dir: dir}
}

// SaveImage writes an image next to the calibration file, e.g. a reference image.
func (c Calibration) SaveImage(name string, img image.Image) error {
	f, err := os.Create(filepath.Join(c.dir, name))
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// LoadCalibration reads the cameras from a JSON file, e.g. darts-o-cam's configuration.
func LoadCalibration(path string) (Calibration, error) {
	var c Calibration
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	c.dir = filepath.Dir(path)
	return c, err
}

// LoadReference loads the reference view of a camera.
func (c Calibration) LoadReference(cam CameraCalibration) (image.Image, error) {
	if cam.Reference == "" {
		return nil, fmt.Errorf("camera %s has no reference image", cam.Name)
	}
	return c.loadImage(cam.Reference)
}

// LoadEmpty loads the image of the empty board of a camera, nil if there is none.
func (c Calibration) LoadEmpty(cam CameraCalibration) (*image.RGBA, error) {
	if cam.Empty == "" {
		return nil, nil
	}
	img, err := c.loadImage(cam.Empty)
	if err != nil {
		return nil, err
	}
	return ToRGBA(img), nil
}

// Dir is the directory of the calibration file, which image paths are relative to.
func (c Calibration) Dir() string {
	return c.dir
}

func (c Calibration) loadImage(name string) (image.Image, error) {
	f, err := os.Open(filepath.Join(c.dir, name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// Camera returns the calibration of the camera with the given name.
func (c Calibration) Camera(name string) (CameraCalibration, error) {
	for _, cam := range c.Cameras {
		if cam.Name == name {
			return cam, nil
		}
	}
	return CameraCalibration{}, fmt.Errorf("camera %q not in calibration", name)
}
