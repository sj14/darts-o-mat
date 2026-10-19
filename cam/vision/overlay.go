package vision

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/sj14/darts-o-mat/common/dartboard"
)

// DrawBoard draws the rings and wires of the board, mapped into the image with m, so a
// calibration can be checked by eye.
func DrawBoard(img image.Image, m Mapping, c color.Color) (*image.RGBA, error) {
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	toImage, err := m.ToBoard.Inverse()
	if err != nil {
		return nil, err
	}
	at := func(r, a float64) dartboard.Point {
		return toImage.Apply(m.Correction.fromBoard(dartboard.Polar(r, a)))
	}

	for _, r := range []float64{dartboard.BullseyeRadius, dartboard.BullRadius, dartboard.TripleInnerRadius,
		dartboard.TripleOuterRadius, dartboard.DoubleInnerRadius, dartboard.DoubleOuterRadius} {
		prev := at(r, 0)
		for a := 1.0; a <= 360; a++ {
			next := at(r, a)
			line(out, prev, next, c)
			prev = next
		}
	}
	for i := range dartboard.Segments {
		a := (float64(i) + 0.5) * dartboard.SegmentAngle
		// In pieces, so the correction bends it like the rings.
		prev := at(dartboard.BullRadius, a)
		for r := dartboard.BullRadius + 5; r <= dartboard.DoubleOuterRadius+5; r += 5 {
			next := at(min(r, dartboard.DoubleOuterRadius), a)
			line(out, prev, next, c)
			prev = next
		}
	}
	return out, nil
}

// DrawCross marks a point, e.g. a detected dart tip.
func DrawCross(img *image.RGBA, p dartboard.Point, size float64, c color.Color) {
	line(img, dartboard.Point{X: p.X - size, Y: p.Y - size}, dartboard.Point{X: p.X + size, Y: p.Y + size}, c)
	line(img, dartboard.Point{X: p.X - size, Y: p.Y + size}, dartboard.Point{X: p.X + size, Y: p.Y - size}, c)
}

func line(img *image.RGBA, a, b dartboard.Point, c color.Color) {
	steps := int(math.Max(math.Abs(b.X-a.X), math.Abs(b.Y-a.Y))) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		img.Set(int(math.Round(a.X+t*(b.X-a.X))), int(math.Round(a.Y+t*(b.Y-a.Y))), c)
	}
}
