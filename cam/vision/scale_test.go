package vision

import (
	"math"
	"testing"
)

func TestPixelScale(t *testing.T) {
	calib, err := LoadCalibration("testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"cam1": 1.435, "cam2": 1.414, "cam3": 1.371}
	var sum float64
	for _, cam := range calib.Cameras {
		toBoard, err := cam.ToBoard()
		if err != nil {
			t.Fatal(err)
		}
		got := PixelScale(toBoard)
		if math.Abs(got-want[cam.Name]) > 0.001 {
			t.Errorf("%s: %.4f px/mm, want %.3f", cam.Name, got, want[cam.Name])
		}
		sum += got
	}
	if mean := sum / 3; math.Abs(mean-ReferenceScale) > 0.001 {
		t.Errorf("mean %.4f px/mm, ReferenceScale is %.4f", mean, ReferenceScale)
	}
}

// At the reference scale, the defaults are exactly the pixel values they were measured as.
func TestDefaultsInPixels(t *testing.T) {
	got := DefaultWatchOptions.pixels(ReferenceScale)
	want := watchParams{
		detect:          detectParams{diff: 25, weak: 12, blur: 2, gap: 3, minPixels: 150, minSpeck: 15, minElongation: 4, axisDistance: 12},
		sampleStep:      4,
		motionThreshold: 40,
		motionSamples:   40,
		takeoutSamples:  4000,
		dartMargin:      5,
	}
	if got != want {
		t.Errorf("defaults at the reference scale = %+v, want %+v", got, want)
	}

	// A camera with half the resolution sees everything half as large.
	half := DefaultWatchOptions.pixels(ReferenceScale / 2)
	if half.detect.blur != 1 || half.detect.minPixels != 38 || half.sampleStep != 2 || half.motionSamples != 40 {
		t.Errorf("at half the scale: %+v", half)
	}
}
