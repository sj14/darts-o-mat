package vision

import (
	"image"
	"image/color"
	_ "image/jpeg"
	"math"
	"os"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
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

			d, err := DetectDart(before, after, DefaultDetectOptions, ReferenceScale)
			if err != nil {
				t.Fatal(err)
			}
			// Blurring fades the last pixels of the thin point, so the tip comes out a few
			// pixels short. Following the axis further in the unblurred images fixed that
			// here, but on real images it ran into the point's shadow and made the results
			// worse (2026-10-11, measured on 5 real throws), so it was dropped.
			if dist := math.Hypot(d.Tip.X-tt.tip.X, d.Tip.Y-tt.tip.Y); dist > 5 {
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

	d, err := DetectDart(before, after, DefaultDetectOptions, ReferenceScale)
	if err != nil {
		t.Fatal(err)
	}
	if dist := math.Hypot(d.Tip.X-tip.X, d.Tip.Y-tip.Y); dist > 5 {
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
	if _, err := DetectDart(before, after, DefaultDetectOptions, ReferenceScale); err != ErrNotElongated {
		t.Errorf("got %v, want ErrNotElongated", err)
	}
}

func TestDetectDartNothingChanged(t *testing.T) {
	img := loadRGBA(t, "testdata/cam3.jpg")
	if _, err := DetectDart(img, img, DefaultDetectOptions, ReferenceScale); err != ErrNoDart {
		t.Errorf("got %v, want ErrNoDart", err)
	}
}

// TestRealDarts checks the calibration of the development board with real darts.
// Their tips were located by hand in each camera image (2026-10-11, two rounds of four);
// the player confirmed the fields. A camera may be missing when it couldn't see the tip
// (a black dart in front of a black segment). Darts within 1.5 mm of a wire are only
// logged: at this resolution they land on either side.
func TestRealDarts(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	maps := map[string]Mapping{}
	for _, cam := range calib.Cameras {
		m, err := cam.Mapping()
		if err != nil {
			t.Fatal(err)
		}
		maps[cam.Name] = m
	}

	darts := []struct {
		want string
		tips map[string]dartboard.Point
	}{
		{"S5", map[string]dartboard.Point{"cam1": {X: 908.7, Y: 211.7}, "cam2": {X: 601.7, Y: 413.3}, "cam3": {X: 570.7, Y: 228.3}}},
		{"S1", map[string]dartboard.Point{"cam1": {X: 788.7, Y: 280.7}, "cam2": {X: 581.7, Y: 260.7}, "cam3": {X: 721, Y: 255}}},
		{"S3", map[string]dartboard.Point{"cam1": {X: 558.3, Y: 256}, "cam2": {X: 722, Y: 149.3}, "cam3": {X: 815, Y: 367.6}}},
		{"S5", map[string]dartboard.Point{"cam1": {X: 734.3, Y: 239.3}, "cam2": {X: 663.3, Y: 252.7}, "cam3": {X: 693.3, Y: 291.7}}},
		{"S8", map[string]dartboard.Point{"cam1": {X: 668.25, Y: 205.5}, "cam2": {X: 753.75, Y: 230}, "cam3": {X: 669.25, Y: 338}}},
		{"S2", map[string]dartboard.Point{"cam1": {X: 588.25, Y: 280}, "cam2": {X: 683.25, Y: 156.25}, "cam3": {X: 824, Y: 340}}},
		{"S18", map[string]dartboard.Point{"cam1": {X: 828, Y: 310}, "cam2": {X: 518, Y: 248}, "cam3": {X: 741, Y: 218}}},
		{"S14", map[string]dartboard.Point{"cam1": {X: 748.75, Y: 121.25}, "cam2": {X: 905.5, Y: 361.25}}},
	}
	for _, dart := range darts {
		var tips []Tip
		for cam, tip := range dart.tips {
			p := maps[cam].Apply(tip)
			radial, tangential := maps[cam].Resolution(p)
			tips = append(tips, Tip{Point: p, Radial: radial, Tangential: tangential})
		}
		p, err := FuseTips(tips)
		if err != nil {
			t.Fatal(err)
		}
		if w := distanceToWire(p); w < 1.5 {
			t.Logf("dart at %.1f,%.1f mm is %.1f mm from a wire, scored %+v (want %s), not checked", p.X, p.Y, w, dartboard.Score(p), dart.want)
			continue
		}
		d := dartboard.Score(p)
		got := "S" + itoa(d.Number)
		if !d.Segment.IsSingle() || got != dart.want {
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
	for y := range 20 {
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

// TestRealThrows runs the detection on real throws recorded with darts-o-cam -save
// (2026-10-11), with the fields confirmed by the player.
func TestRealThrows(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		camera string
		want   dartboard.Hit
	}{
		// A reflection on a wire far below the dart used to be taken for the tip.
		{"003", "cam3", dartboard.Hit{Number: 8, Segment: dartboard.Single}},
		// The barrel in front of black fields only shows as a dashed line.
		{"004", "cam1", dartboard.Hit{Number: 13, Segment: dartboard.Single}},
		// A dart in the bull, recorded while pulling it out (before and after swapped,
		// which doesn't matter for the comparison). Following the axis further used to
		// run 14 px into the shadow, out of the bull.
		{"005", "cam2", dartboard.Hit{Number: dartboard.BullNumber, Segment: dartboard.Bull}},
	}
	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.camera, func(t *testing.T) {
			cam, err := calib.Camera(tt.camera)
			if err != nil {
				t.Fatal(err)
			}
			toBoard, err := cam.ToBoard()
			if err != nil {
				t.Fatal(err)
			}
			before := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-before.jpg")
			after := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-after.jpg")

			// With the camera's own scale, as the watcher does.
			d, err := DetectDart(before, after, DefaultDetectOptions, PixelScale(toBoard))
			if err != nil {
				t.Fatal(err)
			}
			// The field and its multiplier; the side of a single doesn't matter here.
			if got := dartboard.Score(toBoard.Apply(d.Tip)); got.Number != tt.want.Number || got.Segment.Multiplier() != tt.want.Segment.Multiplier() {
				t.Errorf("scored %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestTipOnDartAxis checks that the tip is found at the end of the point (located by
// hand) in cases where it used to be off or missing. The frames come with a newer calibration than
// testdata/calibration.json, so only the pixel is checked.
func TestTipOnDartAxis(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, camera string
		point        dartboard.Point
	}{
		// A 7 at the wire to the 16 (2026-10-14): the pixels at the end of the changed
		// area reached over the wire, their center was 4 px beside the point, in the 16.
		{"033", "cam2", dartboard.Point{X: 687, Y: 387}},
		// A miss just outside the D20, the dart lying flat in the image: the axis of the
		// lower half went through the flight, the tip found was 15 px from the point, in the
		// double.
		{"066", "cam3", dartboard.Point{X: 477, Y: 440}},
		// A T5 passing behind the flight of an earlier dart: only its barrel and point
		// were a piece of their own, too short to count as elongated in the lower half.
		{"004", "cam3", dartboard.Point{X: 613, Y: 345}},
	}
	for _, tt := range tests {
		t.Run(tt.name+"-"+tt.camera, func(t *testing.T) {
			cam, err := calib.Camera(tt.camera)
			if err != nil {
				t.Fatal(err)
			}
			toBoard, err := cam.ToBoard()
			if err != nil {
				t.Fatal(err)
			}
			before := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-before.jpg")
			after := loadRGBA(t, "testdata/throws/"+tt.name+"-"+tt.camera+"-after.jpg")
			d, err := DetectDart(before, after, DefaultDetectOptions, PixelScale(toBoard))
			if err != nil {
				t.Fatal(err)
			}
			if dist := math.Hypot(d.Tip.X-tt.point.X, d.Tip.Y-tt.point.Y); dist > 2 {
				t.Errorf("tip at %.1f,%.1f, %.1f px from the point at %.0f,%.0f", d.Tip.X, d.Tip.Y, dist, tt.point.X, tt.point.Y)
			}
		})
	}
}

// distanceToWire returns how far p is from the nearest ring or wire between numbers, in mm.
func distanceToWire(p dartboard.Point) float64 {
	r := p.Radius()
	d := math.Inf(1)
	for _, ring := range allRingRadii {
		d = min(d, math.Abs(r-ring))
	}
	if r > dartboard.BullRadius {
		off := math.Mod(p.Angle()+dartboard.SegmentAngle/2, dartboard.SegmentAngle)
		angle := min(off, dartboard.SegmentAngle-off) * math.Pi / 180
		d = min(d, r*math.Sin(angle))
	}
	return d
}

func TestFuseTips(t *testing.T) {
	tests := []struct {
		name   string
		points []dartboard.Point
		want   dartboard.Hit
	}{
		// Real T18: two cameras in the triple, one 3 mm inside (2026-10-11).
		{"two of three in the triple", []dartboard.Point{{X: 56.0, Y: 82.3}, {X: 55.6, Y: 78.6}, {X: 59.7, Y: 80.4}}, dartboard.Hit{Number: 18, Segment: dartboard.Triple}},
		// One camera far off doesn't matter.
		{"outlier", []dartboard.Point{{X: 0, Y: 130}, {X: 1, Y: 131}, {X: 40, Y: 60}}, dartboard.Hit{Number: 20, Segment: dartboard.Outer}},
		// Around the center the angles say nothing.
		{"bullseye", []dartboard.Point{{X: 3, Y: 0}, {X: -3, Y: 1}, {X: 0, Y: -2}}, dartboard.Hit{Number: dartboard.BullNumber, Segment: dartboard.Bullseye}},
		// Angles across the wrap-around at ±180° (left of the board, segment 11).
		{"wrap around", []dartboard.Point{{X: -120, Y: 2}, {X: -121, Y: -2}, {X: -119, Y: 0}}, dartboard.Hit{Number: 11, Segment: dartboard.Outer}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := FuseTips(equal(tt.points))
			if err != nil {
				t.Fatal(err)
			}
			if got := dartboard.Score(p); got != tt.want {
				t.Errorf("fused %.1f,%.1f (%.1f mm) scored %+v, want %+v", p.X, p.Y, p.Radius(), got, tt.want)
			}
		})
	}
}

// equal returns tips that all count the same.
func equal(points []dartboard.Point) []Tip {
	tips := make([]Tip, len(points))
	for i, p := range points {
		tips[i] = Tip{Point: p}
	}
	return tips
}

// The cameras count by their resolution at the dart: the squared pixels per mm.
func TestFuseTipsWeighted(t *testing.T) {
	// The real S14 of TestRealDarts: two cameras, the one seeing the dart in the double
	// sees the spot squeezed (0.7 px/mm towards the center).
	far := Tip{Point: dartboard.Polar(166.1, 282.9), Radial: 0.7, Tangential: 1.3}
	near := Tip{Point: dartboard.Polar(157.1, 283.1), Radial: 2.1, Tangential: 1.6}
	p, err := FuseTips([]Tip{far, near})
	if err != nil {
		t.Fatal(err)
	}
	want := (166.1*0.7 + 157.1*2.1) / (0.7 + 2.1)
	if math.Abs(p.Radius()-want) > 0.01 {
		t.Errorf("two cameras: %.2f mm, want the weighted mean %.2f", p.Radius(), want)
	}
	if got := dartboard.Score(p); got != (dartboard.Hit{Number: 14, Segment: dartboard.Outer}) {
		t.Errorf("scored %+v, want S14", got)
	}

	// Three cameras: the weighted median. Two cameras that agree outweigh one that sees
	// the spot better (real miss beside the D6, 2026-10-15: with the resolutions squared,
	// cam1 alone decided D6).
	tips := []Tip{
		{Point: dartboard.Polar(169.8, 87), Radial: 2.2, Tangential: 2.5},
		{Point: dartboard.Polar(176.8, 86), Radial: 1.3, Tangential: 1.2},
		{Point: dartboard.Polar(179.5, 83), Radial: 1.0, Tangential: 1.3},
	}
	if p, _ := FuseTips(tips); math.Abs(p.Radius()-176.8) > 0.01 {
		t.Errorf("three cameras: %.2f mm, want 176.8 (the middle)", p.Radius())
	}
	// A camera that sees the spot better than the two others together still decides.
	tips[0].Radial = 2.5
	if p, _ := FuseTips(tips); math.Abs(p.Radius()-169.8) > 0.01 {
		t.Errorf("three cameras: %.2f mm, want 169.8 (the camera with most of the weight)", p.Radius())
	}

	// A camera whose tip ends on an earlier dart only counts if no other camera found it
	// (real T20 behind two darts in the 20, event 37: cam3 at 91.2 mm).
	hidden := []Tip{
		{Point: dartboard.Polar(98.6, 7), Radial: 1.8, Tangential: 1.1},
		{Point: dartboard.Polar(99.9, 7), Radial: 0.7, Tangential: 1.4},
		{Point: dartboard.Polar(91.2, 14), Radial: 1.8, Tangential: 1.5, Hidden: true},
	}
	want = (98.6*1.8 + 99.9*0.7) / (1.8 + 0.7)
	if p, _ := FuseTips(hidden); math.Abs(p.Radius()-want) > 0.01 {
		t.Errorf("with a hidden camera: %.2f mm, want %.2f (the other two)", p.Radius(), want)
	}
	if p, _ := FuseTips(hidden[2:]); math.Abs(p.Radius()-91.2) > 0.01 {
		t.Errorf("only a hidden camera: %.2f mm, want its 91.2", p.Radius())
	}
}

// Two cameras on opposite sides of the bull don't make a mean neither of them saw: the
// third dart into a group in the 14 (2026-10-22, event 211, the tips as darts-o-cam
// found them live). cam1 saw the 17, cam2 only the upper part of the dart, and cam3's
// tip, the right one, ended on an earlier dart; without it the result was a 16.
func TestFuseTipsDisagreeingPair(t *testing.T) {
	tips := []Tip{
		{Point: dartboard.Point{X: 8.2, Y: -23.5}, Radial: 1.0, Tangential: 1.6},
		{Point: dartboard.Point{X: -63.2, Y: 28.9}, Radial: 1.0, Tangential: 1.5},
		{Point: dartboard.Point{X: -31.1, Y: 11.4}, Radial: 1.7, Tangential: 1.0, Hidden: true},
	}
	p, err := FuseTips(tips)
	if err != nil {
		t.Fatal(err)
	}
	if got := dartboard.Score(p); got != (dartboard.Hit{Number: 14, Segment: dartboard.Inner}) {
		t.Errorf("fused %.1f,%.1f (%.1f mm) scored %+v, want S14", p.X, p.Y, p.Radius(), got)
	}

	// Two cameras that agree well enough still leave the hidden one out (event 37 above,
	// cam1 and cam2 1.3 mm apart).
	tips[1].Point = dartboard.Point{X: 8.0, Y: -2.0}
	if p, _ := FuseTips(tips); p.X < 0 {
		t.Errorf("fused %.1f,%.1f, want between the two visible cameras", p.X, p.Y)
	}
}

// TestBlur compares the running-sum blur with averaging each neighborhood directly.
func TestBlur(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 13, 9))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 37 % 251)
	}
	for _, r := range []int{0, 1, 2, 5} {
		got := blur(img, r)
		w, h := 13, 9
		for y := range h {
			for x := range w {
				for c := range 3 {
					// Rows first, truncated like the implementation, then columns.
					sum, n := 0, 0
					for yy := max(y-r, 0); yy <= min(y+r, h-1); yy++ {
						rowSum, rowN := 0, 0
						for xx := max(x-r, 0); xx <= min(x+r, w-1); xx++ {
							rowSum += int(img.Pix[img.PixOffset(xx, yy)+c])
							rowN++
						}
						sum += rowSum / rowN
						n++
					}
					if want := uint8(sum / n); got.Pix[got.PixOffset(x, y)+c] != want {
						t.Fatalf("r=%d at %d,%d channel %d: got %d, want %d", r, x, y, c, got.Pix[got.PixOffset(x, y)+c], want)
					}
				}
			}
		}
	}
}

// TestFuseTipsDropsOutlier: a real T13 next to a dart already in the board (2026-10-11).
// cam3 saw only part of the new dart behind the old one (35 mm off); the result must
// be the middle of the other two instead of the one closer to cam3.
func TestFuseTipsDropsOutlier(t *testing.T) {
	cam1, cam2, cam3 := dartboard.Point{X: 93.9, Y: 24.9}, dartboard.Point{X: 99.6, Y: 15.4}, dartboard.Point{X: 60.0, Y: 33.0}
	p, err := FuseTips(equal([]dartboard.Point{cam1, cam2, cam3}))
	if err != nil {
		t.Fatal(err)
	}
	if want := (cam1.Radius() + cam2.Radius()) / 2; math.Abs(p.Radius()-want) > 0.01 {
		t.Errorf("radius %.2f mm, want %.2f (mean of cam1 and cam2)", p.Radius(), want)
	}
	if got := dartboard.Score(p).Number; got != 13 {
		t.Errorf("segment %d, want 13", got)
	}
}

// A speck on the dart's axis below its tip isn't part of the dart (2026-10-13,
// darts-o-cam event 30, a dart just outside the double 5): color noise at the triple
// ring's edge, 56 px below the tip, was taken for the tip and scored 12, twice at the
// same spot. Checked in pixels: the frames belong to a newer calibration than testdata's.
func TestDetectIgnoresSpeckBelowTip(t *testing.T) {
	before := loadRGBA(t, "testdata/throws/006-cam2-before.jpg")
	after := loadRGBA(t, "testdata/throws/006-cam2-after.jpg")
	d, err := DetectDart(before, after, DefaultDetectOptions, ReferenceScale)
	if err != nil {
		t.Fatal(err)
	}
	want := dartboard.Point{X: 550, Y: 212} // where the point enters the board
	if dist := math.Hypot(d.Tip.X-want.X, d.Tip.Y-want.Y); dist > 4 {
		t.Errorf("tip at %.0f,%.0f px, want %.0f,%.0f (off by %.0f px)", d.Tip.X, d.Tip.Y, want.X, want.Y, dist)
	}
}

// A dart hit by the next one moves a little; its outline then changes too, merges with
// the new dart and makes the change too wide to be a dart. Real D1 next to a dart in
// the 1 (2026-10-13, cam1 of event 95, after 93 and 94 since the takeout): with the
// earlier darts known, the detection leaves them out.
func TestDetectDartBesideMovedDart(t *testing.T) {
	frames := func(n string) (*image.RGBA, *image.RGBA) {
		return loadRGBA(t, "testdata/throws/"+n+"-cam1-before.jpg"), loadRGBA(t, "testdata/throws/"+n+"-cam1-after.jpg")
	}
	var earlier []image.Point
	for _, n := range []string{"093", "094"} {
		before, after := frames(n)
		_, area, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier)
		if err != nil {
			t.Fatalf("event %s: %v", n, err)
		}
		earlier = append(earlier, area...)
	}

	before, after := frames("095")
	if _, err := DetectDart(before, after, DefaultDetectOptions, ReferenceScale); err != ErrNotElongated {
		t.Fatalf("without the earlier darts: %v, want ErrNotElongated", err)
	}
	d, _, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier)
	if err != nil {
		t.Fatalf("with the earlier darts: %v", err)
	}
	want := dartboard.Point{X: 985, Y: 320} // where the point enters the board, at the double's wire
	if dist(d.Tip, want) > 4 {
		t.Errorf("tip at %.0f,%.0f px, want %.0f,%.0f", d.Tip.X, d.Tip.Y, want.X, want.Y)
	}
	if d.Hidden {
		t.Error("the tip is free next to the earlier dart, not hidden by it")
	}
}

// The third dart into a group in the 14 (2026-10-22, events 209–211, cam3): its point
// ends on the earlier darts, so the tip found is right but counts as hidden; see
// TestFuseTipsDisagreeingPair for why it still decides.
func TestDetectDartBesideHiddenInGroup(t *testing.T) {
	frames := func(n string) (*image.RGBA, *image.RGBA) {
		return loadRGBA(t, "testdata/throws/"+n+"-cam3-before.jpg"), loadRGBA(t, "testdata/throws/"+n+"-cam3-after.jpg")
	}
	var earlier []image.Point
	for _, n := range []string{"209", "210"} {
		before, after := frames(n)
		_, area, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier)
		if err != nil {
			t.Fatalf("event %s: %v", n, err)
		}
		earlier = append(earlier, area...)
	}
	before, after := frames("211")
	d, _, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier)
	if err != nil {
		t.Fatal(err)
	}
	want := dartboard.Point{X: 670, Y: 309} // where the point enters the board
	if dist(d.Tip, want) > 4 {
		t.Errorf("tip at %.0f,%.0f px, want %.0f,%.0f", d.Tip.X, d.Tip.Y, want.X, want.Y)
	}
	if !d.Hidden {
		t.Error("the tip ends on the earlier darts, want it hidden")
	}
}

// fillRect paints a rectangle.
func fillRect(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

// With the earlier darts' pixels taken out, what is left of the change can be a sliver
// along an earlier dart's edge (2026-10-18, event 199, cam3: 484 px of a change of 10,131
// px, its "tip" 25 mm from where the other cameras saw the dart). It is elongated but far
// too small for a dart, so the camera doesn't count.
func TestDetectDartBesideSliverIsNoDart(t *testing.T) {
	bounds := image.Rect(0, 0, 640, 480)
	before := image.NewRGBA(bounds)
	fillRect(before, bounds, color.RGBA{90, 90, 90, 255})
	after := image.NewRGBA(bounds)
	copy(after.Pix, before.Pix)

	// An earlier dart's blob (wide: the change isn't shaped like a dart), and beside it a
	// thin line of a few hundred px that only the retry without the blob leaves.
	blob := image.Rect(200, 150, 330, 280)
	sliver := image.Rect(330, 214, 330+dartMargin2px()+60, 218)
	fillRect(after, blob, color.RGBA{200, 40, 40, 255})
	fillRect(after, sliver, color.RGBA{200, 40, 40, 255})
	var earlier []image.Point
	for y := blob.Min.Y; y < blob.Max.Y; y++ {
		for x := blob.Min.X; x < blob.Max.X; x++ {
			earlier = append(earlier, image.Pt(x, y))
		}
	}

	_, _, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier)
	if err == nil {
		t.Error("a sliver of a few hundred pixels counted as a dart")
	}
	// The same piece as wide as a real dart (over 1000 px) does.
	big := image.Rect(sliver.Min.X, 214, sliver.Max.X, 232)
	fillRect(after, big, color.RGBA{200, 40, 40, 255})
	if _, _, err := DetectDartBeside(before, after, DefaultDetectOptions, ReferenceScale, earlier); err != nil {
		t.Errorf("a piece of over 1000 px: %v, want a dart", err)
	}
}

// dartMargin2px is how far the earlier darts' pixels are widened, in pixels at ReferenceScale.
func dartMargin2px() int {
	return length(dartMargin, ReferenceScale) + 1
}
