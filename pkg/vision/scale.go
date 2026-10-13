package vision

import (
	"math"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// The detection's thresholds are sizes on the board (mm, mm²), so they fit cameras of
// any resolution and distance; each camera converts them to its pixels with its scale.

// ReferenceScale is the scale (image pixels per board millimetre, see PixelScale) the
// detection's thresholds were measured at: the mean of the development board's three
// cameras at 1280×720 (1.435, 1.414 and 1.371 px/mm). The defaults are the pixel values
// measured there, divided by it.
const ReferenceScale = 1.4067

// PixelScale returns how many image pixels a board millimetre is in a camera's view, on
// average over the board: the square root of the board's area in the image (inside the
// double ring) divided by its area in mm². A camera looking across the board shows its
// near side larger than the far side; this is the mean of both.
func PixelScale(toBoard Homography) float64 {
	toImage, err := toBoard.Inverse()
	if err != nil {
		return ReferenceScale
	}
	const n = 360 // points around the double ring, one per degree
	var area float64
	prev := toImage.Apply(dartboard.Polar(dartboard.DoubleOuterRadius, 0))
	for i := 1; i <= n; i++ {
		p := toImage.Apply(dartboard.Polar(dartboard.DoubleOuterRadius, float64(i)))
		area += prev.X*p.Y - p.X*prev.Y
		prev = p
	}
	mm2 := math.Pi * dartboard.DoubleOuterRadius * dartboard.DoubleOuterRadius
	return math.Sqrt(math.Abs(area) / 2 / mm2)
}

// detectParams are DetectOptions in a camera's pixels.
type detectParams struct {
	diff, weak    uint8
	blur, gap     int
	minPixels     int
	minSpeck      int
	minElongation float64
	axisDistance  int
}

// pixels converts the options for a camera with the given scale (pixels per mm).
func (o DetectOptions) pixels(scale float64) detectParams {
	return detectParams{
		diff:          o.DiffThreshold,
		weak:          o.WeakThreshold,
		blur:          length(o.Blur, scale),
		gap:           length(o.Gap, scale),
		minPixels:     area(o.MinArea, scale),
		minSpeck:      area(o.MinSpeck, scale),
		minElongation: o.MinElongation,
		axisDistance:  length(o.AxisDistance, scale),
	}
}

// watchParams are WatchOptions in a camera's pixels.
type watchParams struct {
	detect          detectParams
	sampleStep      int
	motionThreshold int
	// motionSamples and takeoutSamples are areas as numbers of sampled pixels.
	motionSamples, takeoutSamples int
	// dartMargin is how far around a dart its pixels are remembered, see Watcher.rememberDart.
	dartMargin int
}

func (o WatchOptions) pixels(scale float64) watchParams {
	step := max(length(o.SampleStep, scale), 1)
	samples := func(mm2 float64) int {
		return int(math.Round(mm2 * scale * scale / float64(step*step)))
	}
	return watchParams{
		detect:          o.Detect.pixels(scale),
		sampleStep:      step,
		motionThreshold: int(o.MotionThreshold),
		motionSamples:   samples(o.MotionArea),
		takeoutSamples:  samples(o.TakeoutArea),
		dartMargin:      length(dartMargin, scale),
	}
}

// length converts a length in mm to whole pixels.
func length(mm, scale float64) int {
	return int(math.Round(mm * scale))
}

// area converts an area in mm² to whole pixels.
func area(mm2, scale float64) int {
	return int(math.Round(mm2 * scale * scale))
}

// px converts a pixel value measured at ReferenceScale to mm, for the defaults.
func px(pixels float64) float64 {
	return pixels / ReferenceScale
}

// px2 converts an area in pixels measured at ReferenceScale to mm², for the defaults.
func px2(pixels float64) float64 {
	return pixels / (ReferenceScale * ReferenceScale)
}
