package vision

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// renderBoard draws a dartboard with the standard dimensions and colors into an image,
// mapped by toImage (board mm to pixels).
func renderBoard(toImage Homography, w, h int) *image.RGBA {
	toBoard, _ := toImage.Inverse()
	var (
		red   = color.RGBA{200, 50, 60, 255}
		green = color.RGBA{40, 150, 80, 255}
		black = color.RGBA{25, 25, 25, 255}
		white = color.RGBA{235, 230, 215, 255}
		wire  = color.RGBA{190, 190, 190, 255}
	)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			p := toBoard.Apply(dartboard.Point{X: float64(x), Y: float64(y)}) // pixel centers at whole numbers, like vision samples them
			r := p.Radius()
			i := dartboard.SegmentIndex(p.Number())
			offWire := math.Abs(math.Mod(p.Angle()+dartboard.SegmentAngle/2, dartboard.SegmentAngle) - 0) // distance past a wire
			onWire := r > dartboard.BullRadius && r < dartboard.DoubleOuterRadius && (offWire < 0.3 || offWire > dartboard.SegmentAngle-0.3)
			c := black
			switch {
			case r <= dartboard.BullseyeRadius:
				c = red
			case r <= dartboard.BullRadius:
				c = green
			case onWire:
				c = wire
			case r > dartboard.DoubleOuterRadius:
				c = black
			case r >= dartboard.DoubleInnerRadius || (r >= dartboard.TripleInnerRadius && r <= dartboard.TripleOuterRadius):
				c = red
				if i%2 == 1 {
					c = green
				}
			default:
				c = black
				if i%2 == 1 {
					c = white
				}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestRefineCalibration(t *testing.T) {
	// The view of one of the development board's cameras.
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam3")
	camToBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}
	truth, _ := camToBoard.Inverse()
	img := renderBoard(truth, 1280, 720)

	// A rough calibration: rotated by 1° around the bull and shifted by a few pixels.
	c := truth.Apply(dartboard.Point{})
	rot := 1.0 * math.Pi / 180
	move := Homography{math.Cos(rot), -math.Sin(rot), 0, math.Sin(rot), math.Cos(rot), 0, 0, 0, 1}
	move = Homography{1, 0, c.X + 3, 0, 1, c.Y - 2, 0, 0, 1}.Mul(move).Mul(Homography{1, 0, -c.X, 0, 1, -c.Y, 0, 0, 1})
	rough := move.Mul(truth)
	roughToBoard, _ := rough.Inverse()

	res, err := RefineCalibration(img, roughToBoard)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("matches %d, rms %.2f px before, %.2f px after", res.Matches, res.RMSBefore, res.RMSAfter)

	truthToBoard, _ := truth.Inverse()
	var worstBefore, worstAfter float64
	for _, p := range []dartboard.Point{{X: 0, Y: 0}, dartboard.Polar(103, 0), dartboard.Polar(166, 100), dartboard.Polar(166, 250), dartboard.Polar(60, 180)} {
		px := truth.Apply(p)
		worstBefore = max(worstBefore, dist(roughToBoard.Apply(px), truthToBoard.Apply(px)))
		worstAfter = max(worstAfter, dist(res.ToBoard.Apply(px), truthToBoard.Apply(px)))
	}
	t.Logf("worst error on the board: %.2f mm before, %.2f mm after", worstBefore, worstAfter)
	if worstAfter > 0.5 {
		t.Errorf("refined calibration is off by up to %.2f mm, want at most 0.5", worstAfter)
	}

	// The drawn board has its edges exactly at the wires, and no distortion: there is
	// nothing to correct, apart from noise of the edges (aliased, at the far side less
	// than a pixel per mm).
	if s := res.EdgeShift; math.Abs(s.Red) > 0.2 || math.Abs(s.Green) > 0.2 {
		t.Errorf("edge shift red %.2f, green %.2f mm, want about 0", s.Red, s.Green)
	}
	m := res.Mapping()
	var worst float64
	for i, ring := range m.Correction.Offsets {
		for k, o := range ring {
			if math.Abs(o) > math.Abs(worst) {
				worst = o
			}
			if math.Abs(o) > 0.5 {
				t.Fatalf("correction of the ring at %.1f mm is %.2f mm at %d°, want about 0", m.Correction.Radii[i], o, k*correctionStep)
			}
		}
	}
	t.Logf("largest correction %.2f mm", worst)
}

// TestRefineCalibrationReal refines the development board's calibration on real images of
// the empty board, from a deliberately rough start (rotated by 4°, shifted by 10 px, up
// to ~30 mm off). It must end up where the refinement from the hand-clicked points does.
func TestRefineCalibrationReal(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	probe := []dartboard.Point{{X: 0, Y: 0}, dartboard.Polar(103, 0), dartboard.Polar(166, 90), dartboard.Polar(166, 180), dartboard.Polar(166, 270)}
	for _, cam := range calib.Cameras {
		t.Run(cam.Name, func(t *testing.T) {
			img := loadRGBA(t, "testdata/"+cam.Name+"-empty.jpg")
			points, err := cam.FromPoints()
			if err != nil {
				t.Fatal(err)
			}
			good, err := RefineCalibration(img, points)
			if err != nil {
				t.Fatal(err)
			}
			if good.RMSAfter > 2 {
				t.Errorf("deviation after refining %.2f px, want at most 2", good.RMSAfter)
			}
			// The red rings' edges are found in the black fields, see RefineCalibration.
			t.Logf("edge shift red %.2f, green %.2f mm", good.EdgeShift.Red, good.EdgeShift.Green)
			if s := good.EdgeShift; s.Red < 0.3 || s.Red-s.Green < 0.3 {
				t.Errorf("edge shift red %.2f, green %.2f mm: want the red rings found wider", s.Red, s.Green)
			}

			toImage, _ := points.Inverse()
			c := toImage.Apply(dartboard.Point{})
			rot := 4 * math.Pi / 180
			move := Homography{math.Cos(rot), -math.Sin(rot), 0, math.Sin(rot), math.Cos(rot), 0, 0, 0, 1}
			move = Homography{1, 0, c.X + 10, 0, 1, c.Y - 10, 0, 0, 1}.Mul(move).Mul(Homography{1, 0, -c.X, 0, 1, -c.Y, 0, 0, 1})
			rough, _ := move.Mul(toImage).Inverse()
			res, err := RefineCalibration(img, rough)
			if err != nil {
				t.Fatal(err)
			}

			goodImage, _ := good.ToBoard.Inverse()
			for _, p := range probe {
				px := goodImage.Apply(p)
				if d := dist(res.ToBoard.Apply(px), p); d > 0.5 {
					t.Errorf("at %.0f,%.0f mm the rough start ends up %.2f mm off", p.X, p.Y, d)
				}
			}
		})
	}
}
