package vision

import "testing"

// TestTraceDart checks that the trace comes to the same tip as the detection and records
// its steps.
func TestTraceDart(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, err := calib.Camera("cam3")
	if err != nil {
		t.Fatal(err)
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		t.Fatal(err)
	}
	before := loadRGBA(t, "testdata/throws/066-cam3-before.jpg")
	after := loadRGBA(t, "testdata/throws/066-cam3-after.jpg")
	scale := PixelScale(toBoard)

	d, err := DetectDart(before, after, DefaultDetectOptions, scale)
	if err != nil {
		t.Fatal(err)
	}
	tr := TraceDart(before, after, DefaultDetectOptions, scale, nil)
	if tr.Err != nil || tr.Tip.Tip != d.Tip {
		t.Fatalf("trace: tip %v, %v; detection: %v", tr.Tip.Tip, tr.Err, d.Tip)
	}
	if tr.Changed == nil || len(tr.Pieces) == 0 || len(tr.Seed) == 0 || len(tr.Area) < len(tr.Seed) || len(tr.TipEnd) == 0 || tr.Elongation < DefaultDetectOptions.MinElongation {
		t.Errorf("steps missing: %d pieces, seed %d px, area %d px, tip end %d px, elongation %.1f", len(tr.Pieces), len(tr.Seed), len(tr.Area), len(tr.TipEnd), tr.Elongation)
	}
	if img := DrawTrace(after, tr); img.Bounds() != after.Bounds() || img.RGBAAt(int(d.Tip.X), int(d.Tip.Y)) != TraceTipColor {
		t.Error("the drawn trace has no tip cross")
	}
}
