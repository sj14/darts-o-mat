package vision

import (
	"image"
	"image/color"
	_ "image/jpeg"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

func loadRGBA(t *testing.T, path string) *image.RGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// Copy, so drawing on it doesn't change other tests' images.
	rgba := ToRGBA(img)
	c := image.NewRGBA(rgba.Bounds())
	copy(c.Pix, rgba.Pix)
	return c
}

// drawDart draws a simplified dart: a thin point at tip, a barrel, a shaft and a
// wide flight, pointing from the tip in direction dir.
func drawDart(img *image.RGBA, tip dartboard.Point, dir dartboard.Point) {
	n := math.Hypot(dir.X, dir.Y)
	dir.X, dir.Y = dir.X/n, dir.Y/n
	parts := []struct {
		from, to float64 // distance from the tip
		width    float64
		color    color.RGBA
	}{
		// Sizes as seen by the development board's cameras: the flight is large.
		{0, 35, 2, color.RGBA{170, 170, 175, 255}},   // steel point
		{35, 120, 9, color.RGBA{150, 150, 155, 255}}, // barrel
		{120, 160, 4, color.RGBA{40, 40, 40, 255}},   // shaft
		{160, 230, 60, color.RGBA{30, 60, 170, 255}}, // flight
	}
	for _, part := range parts {
		for d := part.from; d <= part.to; d += 0.5 {
			cx, cy := tip.X+dir.X*d, tip.Y+dir.Y*d
			for o := -part.width / 2; o <= part.width/2; o += 0.5 {
				x, y := int(math.Round(cx-dir.Y*o)), int(math.Round(cy+dir.X*o))
				if image.Pt(x, y).In(img.Bounds()) {
					img.SetRGBA(x, y, part.color)
				}
			}
		}
	}
}

func TestDetectDart(t *testing.T) {
	// The cameras look across the board, so darts point up in the image.
	tests := []struct {
		name string
		tip  dartboard.Point
		dir  dartboard.Point
	}{
		{"leaning left", dartboard.Point{X: 700, Y: 400}, dartboard.Point{X: -0.3, Y: -1}},
		{"leaning right", dartboard.Point{X: 600, Y: 300}, dartboard.Point{X: 0.5, Y: -1}},
		{"in the bull", dartboard.Point{X: 725, Y: 310}, dartboard.Point{X: 0.2, Y: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := loadRGBA(t, "testdata/cam3.jpg")
			after := loadRGBA(t, "testdata/cam3.jpg")
			drawDart(after, tt.tip, tt.dir)

			d, err := DetectDart(before, after, DefaultDetectOptions)
			if err != nil {
				t.Fatal(err)
			}
			if dist := math.Hypot(d.Tip.X-tt.tip.X, d.Tip.Y-tt.tip.Y); dist > 2 {
				t.Errorf("tip = %.1f,%.1f, want %.0f,%.0f (off by %.1f px)", d.Tip.X, d.Tip.Y, tt.tip.X, tt.tip.Y, dist)
			}
		})
	}
}

func TestDetectDartIgnoresSpecks(t *testing.T) {
	before := loadRGBA(t, "testdata/cam3.jpg")
	after := loadRGBA(t, "testdata/cam3.jpg")
	tip := dartboard.Point{X: 640, Y: 360}
	drawDart(after, tip, dartboard.Point{X: -0.1, Y: -1})
	// A small change far below the dart, e.g. a reflection on a wire.
	for y := 495; y < 501; y++ {
		for x := 495; x < 501; x++ {
			after.SetRGBA(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	d, err := DetectDart(before, after, DefaultDetectOptions)
	if err != nil {
		t.Fatal(err)
	}
	if dist := math.Hypot(d.Tip.X-tip.X, d.Tip.Y-tip.Y); dist > 2 {
		t.Errorf("tip = %.1f,%.1f, want %.0f,%.0f", d.Tip.X, d.Tip.Y, tip.X, tip.Y)
	}
}

// Only the flight is visible, e.g. the rest is hidden behind other darts: there's no tip to find.
func TestDetectDartOnlyFlight(t *testing.T) {
	before := loadRGBA(t, "testdata/cam3.jpg")
	after := loadRGBA(t, "testdata/cam3.jpg")
	for y := 100; y < 170; y++ {
		for x := 600; x < 660; x++ {
			after.SetRGBA(x, y, color.RGBA{30, 60, 170, 255})
		}
	}
	if _, err := DetectDart(before, after, DefaultDetectOptions); err != ErrNotElongated {
		t.Errorf("got %v, want ErrNotElongated", err)
	}
}

func TestDetectDartNothingChanged(t *testing.T) {
	img := loadRGBA(t, "testdata/cam3.jpg")
	if _, err := DetectDart(img, img, DefaultDetectOptions); err != ErrNoDart {
		t.Errorf("got %v, want ErrNoDart", err)
	}
}

// TestRealDarts checks the calibration of the development board with real darts.
// Their tips were located by hand in each camera image (2026-10-11, two rounds of four);
// the player confirmed the fields. One more dart, a single 5 right next to the wire
// to the 12, ends up 1 mm on the wrong side and is left out. A camera may be missing
// when it couldn't see the tip (a black dart in front of a black segment).
func TestRealDarts(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	toBoard := map[string]Homography{}
	for _, cam := range calib.Cameras {
		h, err := cam.ToBoard()
		if err != nil {
			t.Fatal(err)
		}
		toBoard[cam.Name] = h
	}

	darts := []struct {
		want string
		tips map[string]dartboard.Point
	}{
		{"S5", map[string]dartboard.Point{"cam1": {X: 908.7, Y: 211.7}, "cam2": {X: 601.7, Y: 413.3}, "cam3": {X: 570.7, Y: 228.3}}},
		{"S1", map[string]dartboard.Point{"cam1": {X: 788.7, Y: 280.7}, "cam2": {X: 581.7, Y: 260.7}, "cam3": {X: 721, Y: 255}}},
		{"S3", map[string]dartboard.Point{"cam1": {X: 558.3, Y: 256}, "cam2": {X: 722, Y: 149.3}, "cam3": {X: 815, Y: 367.6}}},
		{"S8", map[string]dartboard.Point{"cam1": {X: 668.25, Y: 205.5}, "cam2": {X: 753.75, Y: 230}, "cam3": {X: 669.25, Y: 338}}},
		{"S2", map[string]dartboard.Point{"cam1": {X: 588.25, Y: 280}, "cam2": {X: 683.25, Y: 156.25}, "cam3": {X: 824, Y: 340}}},
		{"S18", map[string]dartboard.Point{"cam1": {X: 828, Y: 310}, "cam2": {X: 518, Y: 248}, "cam3": {X: 741, Y: 218}}},
		{"S14", map[string]dartboard.Point{"cam1": {X: 748.75, Y: 121.25}, "cam2": {X: 905.5, Y: 361.25}}},
	}
	for _, dart := range darts {
		var points []dartboard.Point
		for cam, tip := range dart.tips {
			points = append(points, toBoard[cam].Apply(tip))
		}
		p, err := FuseTips(points)
		if err != nil {
			t.Fatal(err)
		}
		d := dartboard.Score(p)
		got := "S" + itoa(d.Number)
		if d.Multiplier != 1 || got != dart.want {
			t.Errorf("dart at %.1f,%.1f mm scored %+v, want %s", p.X, p.Y, d, dart.want)
		}
	}
}

func itoa(n uint8) string {
	if n >= 10 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	return string(rune('0' + n))
}

func TestComponentsBridging(t *testing.T) {
	m := &Mask{W: 20, H: 20, Pix: make([]bool, 400)}
	// A dashed vertical line: pieces of 3 px with gaps of 4 px.
	for y := 0; y < 20; y++ {
		if y%7 < 3 {
			m.Pix[y*20+10] = true
		}
	}
	// A single pixel far away.
	m.Pix[2*20+1] = true

	if got := len(m.components()); got != 4 {
		t.Errorf("components = %d, want 4 (3 dashes and the pixel)", got)
	}
	comps := m.componentsBridging(3)
	if len(comps) != 2 {
		t.Fatalf("bridged components = %d, want 2", len(comps))
	}
	sizes := []int{len(comps[0]), len(comps[1])}
	slices.Sort(sizes)
	// Only the mask's own pixels: 9 of the line and the single one.
	if !slices.Equal(sizes, []int{1, 9}) {
		t.Errorf("sizes = %v, want [1 9]", sizes)
	}
}

// TestRealThrows runs the detection on real throws recorded with darts-o-cam detect -save
// (2026-10-11), with the fields confirmed by the player.
func TestRealThrows(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		camera string
		want   throw.Dart
	}{
		// A reflection on a wire far below the dart used to be taken for the tip.
		{"003", "cam3", throw.Dart{Number: 8, Multiplier: 1}},
		// The barrel in front of black fields only shows as a dashed line.
		{"004", "cam1", throw.Dart{Number: 13, Multiplier: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.camera, func(t *testing.T) {
			cam, err := findCamera(calib, tt.camera)
			if err != nil {
				t.Fatal(err)
			}
			toBoard, err := cam.ToBoard()
			if err != nil {
				t.Fatal(err)
			}
			before := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-before.jpg")
			after := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-after.jpg")

			d, err := DetectDart(before, after, DefaultDetectOptions)
			if err != nil {
				t.Fatal(err)
			}
			if got := dartboard.Score(toBoard.Apply(d.Tip)); got != tt.want {
				t.Errorf("scored %+v, want %+v", got, tt.want)
			}
		})
	}
}
