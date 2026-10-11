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
	// Device fixes the capture device by its camera.Device ID. Usually it is empty and the
	// camera is found by the calibration's DeviceName and its Reference view, because the
	// IDs change with the USB port and identical cameras can't be told apart otherwise.
	Device string `json:"device,omitempty"`
	// Reference is an image of the camera's view, relative to the calibration file.
	Reference string `json:"reference,omitempty"`
	// Empty is an image of the empty board, relative to the calibration file. It lets the
	// detection tell darts pulled out from darts thrown (see Watcher.Empty).
	Empty string `json:"empty,omitempty"`
	// Points are the rough calibration, e.g. clicked by hand.
	Points []CalibrationPoint `json:"points"`
	// Refined is the calibration improved by RefineCalibration (image pixels to board mm).
	// If set, it is used instead of the points.
	Refined *Homography `json:"refined,omitempty"`
}

// ToBoard returns the homography mapping image pixels to board millimeters:
// the refined one if there is one, otherwise the one from the points.
func (c CameraCalibration) ToBoard() (Homography, error) {
	if c.Refined != nil {
		return *c.Refined, nil
	}
	return c.FromPoints()
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

type Calibration struct {
	// DeviceName is the name of the board's capture devices, e.g. "Autodarts DIY Cam".
	// It selects the board's cameras independent of the USB port; which camera is which
	// is told by its view (see AssignCameras).
	DeviceName string `json:"device_name,omitempty"`
	// DeviceModel is the USB vendor:product id of the devices, e.g. "1e45:8022",
	// to recognize them when shown; it is not used to select them.
	DeviceModel string              `json:"device_model,omitempty"`
	Cameras     []CameraCalibration `json:"cameras"`

	dir string // of the calibration file, for the reference images
}

// Save writes the calibration as JSON.
func (c Calibration) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// NewCalibration starts an empty calibration that will be saved at path.
func NewCalibration(path, deviceName string) Calibration {
	return Calibration{DeviceName: deviceName, dir: filepath.Dir(path)}
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
		return nil, fmt.Errorf("camera %s has neither a device nor a reference image", cam.Name)
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
