package vision

import (
	"encoding/json"
	"fmt"
	"os"

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
	Name   string             `json:"name"`
	Device string             `json:"device"` // capture device, e.g. the AVFoundation index
	Points []CalibrationPoint `json:"points"`
}

// ToBoard returns the homography mapping image pixels to board millimeters.
func (c CameraCalibration) ToBoard() (Homography, error) {
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
	Cameras []CameraCalibration `json:"cameras"`
}

func LoadCalibration(path string) (Calibration, error) {
	var c Calibration
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	return c, err
}
