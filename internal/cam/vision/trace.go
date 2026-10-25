package vision

import (
	"image"
	"image/color"
	"math"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// Trace records the steps of a camera's dart detection, to show them (DrawTrace) when a
// detection looks wrong. Only TraceDart records one; the detection itself doesn't.
type Trace struct {
	Changed *Mask // the pixels that changed (Diff)
	Earlier *Mask // the changed pixels of darts thrown earlier (widened if Retried), nil without
	// Retried is set when the change wasn't shaped like a dart and the detection ran
	// again without the changes at earlier darts (see detect); the rest is that run's.
	Retried bool
	// Pieces are the changed pieces, gaps bridged, without specks; largest first.
	Pieces [][]image.Point
	// Seed is the piece taken for the dart's barrel and point, Area the seed with the
	// pieces on its axis (e.g. the flight); nil seed: no piece was elongated enough.
	Seed, Area []image.Point
	// Elongation is the seed's (length to width), or the largest piece's without a seed.
	Elongation float64
	// FirstAxis is the axis of the seed's lower half, Axis the one fitted again to the
	// part near the tip, which the tip lies on (see tipAxis).
	FirstAxis, Axis Line
	// TipEnd are the pixels at the lower end the tip is averaged from.
	TipEnd []image.Point
	Tip    Detection
	Err    error
}

// Line is a line in the image: a point on it and its direction (unit vector).
type Line struct {
	Point, Dir dartboard.Point
}

// TraceDart is DetectDartBeside recording its steps.
func TraceDart(before, after *image.RGBA, opts DetectOptions, scale float64, earlier []image.Point) Trace {
	var tr Trace
	p := opts.pixels(scale)
	_, changed := p.compare(before, after)
	tr.Tip, _, tr.Err = detect(changed, p, dartsMask(before.Bounds(), earlier), &tr)
	return tr
}

// Colors of DrawTrace.
var (
	TraceChanged  = color.RGBA{120, 120, 120, 255} // changed, but a speck
	TracePiece    = color.RGBA{60, 140, 255, 255}  // a piece that isn't part of the dart
	TraceArea     = color.RGBA{255, 60, 60, 255}   // part of the dart, beside the seed
	TraceSeed     = color.RGBA{255, 200, 0, 255}   // the seed: barrel and point
	TraceEarlier  = color.RGBA{0, 220, 220, 255}   // earlier darts
	TraceFirst    = color.RGBA{255, 0, 255, 255}   // the lower half's axis
	TraceAxis     = color.RGBA{0, 255, 255, 255}   // the axis the tip lies on
	TraceTipEnd   = color.RGBA{255, 255, 255, 255} // the pixels the tip is averaged from
	TraceTipColor = color.RGBA{0, 255, 0, 255}     // the tip
)

// DrawTrace draws the steps of a detection onto a copy of the frame after the dart.
func DrawTrace(after *image.RGBA, tr Trace) *image.RGBA {
	img := image.NewRGBA(after.Bounds())
	copy(img.Pix, after.Pix)
	if tr.Earlier != nil {
		tint(img, maskPoints(tr.Earlier), TraceEarlier, 0.25)
	}
	if tr.Changed != nil {
		tint(img, maskPoints(tr.Changed), TraceChanged, 0.5)
	}
	for _, p := range tr.Pieces {
		tint(img, p, TracePiece, 0.6)
	}
	tint(img, tr.Area, TraceArea, 0.6)
	tint(img, tr.Seed, TraceSeed, 0.6)
	if tr.Seed != nil {
		// Along the dart and a little beyond its end, not over the board below the tip,
		// which the zoomed image is for.
		drawAxis(img, tr.FirstAxis, tr.Area, TraceFirst)
		drawAxis(img, tr.Axis, tr.Area, TraceAxis)
	}
	tint(img, tr.TipEnd, TraceTipEnd, 0.8)
	if tr.Err == nil {
		DrawCross(img, tr.Tip.Tip, 10, TraceTipColor)
	}
	return img
}

// tint blends c into the pixels.
func tint(img *image.RGBA, pts []image.Point, c color.RGBA, alpha float64) {
	for _, p := range pts {
		if !p.In(img.Rect) {
			continue
		}
		o := img.RGBAAt(p.X, p.Y)
		mix := func(a, b uint8) uint8 { return uint8(float64(a)*(1-alpha) + float64(b)*alpha) }
		img.SetRGBA(p.X, p.Y, color.RGBA{mix(o.R, c.R), mix(o.G, c.G), mix(o.B, c.B), 255})
	}
}

// drawAxis draws the axis along the dart's pixels, to axisBeyond past their end.
func drawAxis(img *image.RGBA, l Line, dart []image.Point, c color.RGBA) {
	from, to := math.Inf(1), math.Inf(-1)
	for _, p := range dart {
		s := (float64(p.X)-l.Point.X)*l.Dir.X + (float64(p.Y)-l.Point.Y)*l.Dir.Y
		from, to = min(from, s), max(to, s)
	}
	for s := from; s <= to+axisBeyond; s += 0.5 {
		x, y := l.Point.X+s*l.Dir.X, l.Point.Y+s*l.Dir.Y
		if p := image.Pt(int(math.Round(x)), int(math.Round(y))); p.In(img.Rect) {
			img.SetRGBA(p.X, p.Y, c)
		}
	}
}

// axisBeyond is how far an axis is drawn past the end of the dart, in pixels.
const axisBeyond = 8

func maskPoints(m *Mask) []image.Point {
	var pts []image.Point
	for i, set := range m.Pix {
		if set {
			pts = append(pts, image.Pt(i%m.W, i/m.W))
		}
	}
	return pts
}
