package vision

import (
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// autoDeviation is how far a calibration is from the truth, in mm on the board: the mean
// over points on four rings all around.
func autoDeviation(m Mapping, truthToImage Homography) float64 {
	var sum float64
	var n int
	for _, radius := range []float64{50, 100, 135, 166} {
		for k := range 36 {
			board := dartboard.Polar(radius, float64(k)*10+3)
			got := m.Apply(truthToImage.Apply(board))
			sum += math.Hypot(got.X-board.X, got.Y-board.Y)
			n++
		}
	}
	return sum / float64(n)
}

// The development board's three cameras, calibrated by hand and refined, are the truth: one
// tap anywhere in the double of the 20 has to lead to the same calibration, whichever of
// them is looked at and wherever in the segment (±9°, between the rings) the tap is.
func TestAutoCalibrateReal(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	taps := []struct{ angle, radius float64 }{{0, 166}, {-8, 163}, {8, 169}}
	for _, name := range []string{"cam1", "cam2", "cam3"} {
		cam, _ := calib.Camera(name)
		truth, err := cam.ToBoard()
		if err != nil {
			t.Fatal(err)
		}
		truthToImage, _ := truth.Inverse()
		img, err := calib.LoadEmpty(cam)
		if err != nil {
			t.Fatal(err)
		}
		for _, tap := range taps {
			res, err := AutoCalibrate(img, truthToImage.Apply(dartboard.Polar(tap.radius, tap.angle)))
			if err != nil {
				t.Errorf("%s, tap at %+.0f°: %v", name, tap.angle, err)
				continue
			}
			if share := res.FoundShare(); share < 0.8 {
				t.Errorf("%s, tap at %+.0f°: found %.0f %% of the features, want at least 80 %%", name, tap.angle, share*100)
			}
			if dev := autoDeviation(res.Mapping(), truthToImage); dev > 1 {
				t.Errorf("%s, tap at %+.0f°: %.2f mm from the calibration by hand, want less than 1 mm", name, tap.angle, dev)
			}
			if len(res.Points) != len(autoPointTargets) {
				t.Errorf("%s: %d points, want %d", name, len(res.Points), len(autoPointTargets))
			}
		}
	}
}

// The points read off a result are as good as clicked ones: refining from them again leads
// to the same calibration.
func TestAutoCalibratePoints(t *testing.T) {
	calib, _ := LoadCalibration("testdata/calibration.json")
	cam, _ := calib.Camera("cam2")
	truth, _ := cam.ToBoard()
	truthToImage, _ := truth.Inverse()
	img, _ := calib.LoadEmpty(cam)

	res, err := AutoCalibrate(img, truthToImage.Apply(dartboard.Polar(166, 0)))
	if err != nil {
		t.Fatal(err)
	}
	rough, err := CameraCalibration{Name: "cam2", Points: res.Points}.FromPoints()
	if err != nil {
		t.Fatal(err)
	}
	again, err := RefineCalibration(img, rough)
	if err != nil {
		t.Fatal(err)
	}
	if dev := autoDeviation(again.Mapping(), truthToImage); dev > 1 {
		t.Errorf("refined again from the points: %.2f mm from the truth", dev)
	}
}

// Where there is no double ring to be found, it says so instead of making something up.
func TestAutoCalibrateNoBoard(t *testing.T) {
	calib, _ := LoadCalibration("testdata/calibration.json")
	cam, _ := calib.Camera("cam1")
	truth, _ := cam.ToBoard()
	truthToImage, _ := truth.Inverse()
	img, _ := calib.LoadEmpty(cam)

	// A tap far from any ring: on the board's black surround, on the number ring, in a corner.
	for _, tap := range []dartboard.Point{{X: 30, Y: 30}, truthToImage.Apply(dartboard.Polar(210, 0)), truthToImage.Apply(dartboard.Polar(40, 90))} {
		if _, err := AutoCalibrate(img, tap); !errors.Is(err, ErrNoBoard) {
			t.Errorf("tap at %v: error %v, want ErrNoBoard", tap, err)
		}
	}

	// A frame with nothing on it, and one that is dark.
	for _, fill := range []color.RGBA{{128, 128, 128, 255}, {5, 5, 5, 255}} {
		blank := image.NewRGBA(image.Rect(0, 0, 640, 480))
		for i := 0; i < len(blank.Pix); i += 4 {
			blank.Pix[i], blank.Pix[i+1], blank.Pix[i+2], blank.Pix[i+3] = fill.R, fill.G, fill.B, 255
		}
		if _, err := AutoCalibrate(blank, dartboard.Point{X: 320, Y: 240}); !errors.Is(err, ErrNoBoard) {
			t.Errorf("blank frame %v: error %v, want ErrNoBoard", fill, err)
		}
	}
}

// A board drawn by the tests' own renderer, seen from the side, is found as well: it has no
// glare and no logos, so this is the geometry alone.
func TestAutoCalibrateSynthetic(t *testing.T) {
	calib, _ := LoadCalibration("testdata/calibration.json")
	cam, _ := calib.Camera("cam3")
	truth, _ := cam.ToBoard()
	truthToImage, _ := truth.Inverse()
	img := renderBoard(truthToImage, 1280, 720)

	res, err := AutoCalibrate(img, truthToImage.Apply(dartboard.Polar(166, 4)))
	if err != nil {
		t.Fatal(err)
	}
	if dev := autoDeviation(res.Mapping(), truthToImage); dev > 0.5 {
		t.Errorf("%.2f mm from the truth, want less than 0.5 mm", dev)
	}
}

// A second board (Winmau Blade 6 again, but in another place), seen at a very flat angle by
// three cameras whose lenses color the edges of the numbers and the wires red and green
// (chromatic aberration): the colored pixels of the double ring then reach over the wires
// and the numbers to the board's rim. The taps are in the double of the 20, which lies
// left, right and below in the three pictures.
func TestAutoCalibrateFlat(t *testing.T) {
	for _, c := range []struct {
		file string
		tap  dartboard.Point
	}{
		{"testdata/flat-cam1.jpg", dartboard.Point{X: 345, Y: 210}},
		{"testdata/flat-cam3.jpg", dartboard.Point{X: 846, Y: 188}},
		{"testdata/flat-cam4.jpg", dartboard.Point{X: 600, Y: 448}},
	} {
		f, err := os.Open(c.file)
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		res, err := AutoCalibrate(img, c.tap)
		if err != nil {
			t.Errorf("%s: %v", c.file, err)
			continue
		}
		if share := res.FoundShare(); share < 0.8 {
			t.Errorf("%s: found %.0f %% of the features, want at least 80 %%", c.file, share*100)
		}
		// The 20 is where the tap was: the rotation is right, not one segment off.
		at, err := res.Mapping().ToImage(dartboard.Polar(166, 0))
		if err != nil {
			t.Fatal(err)
		}
		if d := math.Hypot(at.X-c.tap.X, at.Y-c.tap.Y); d > 30 {
			t.Errorf("%s: the double of the 20 is %.0f px from the tap", c.file, d)
		}
	}
}
