package view

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"

	"github.com/a-h/templ"

	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
)

// The heatmap is a density, not a square per dart: each dart is spread out as a
// Gaussian, so where many land glows and single darts stay faint. Its color is the
// share of the highest density; by its square root, the spread of 759 darts glowed all
// over the board.

// heatPixel is the side of a pixel of the heatmap's image in mm: a cell of stats.Heat,
// so each cell is a pixel.
const heatPixel = stats.HeatCellSize

// heatBoardRadius is how far the image reaches from the bull (mm): the drawn board's
// edge, so the spread of darts near HeatReach isn't cut off by the image's border.
const heatBoardRadius = boardCenter - 2

// heatHalf is how many pixels lie on each side of the bull; the bull is at a corner
// between pixels, as cells start at 0 mm.
var heatHalf = int(math.Ceil(heatBoardRadius / heatPixel))

// heatSpread is how far a dart is spread out (mm, the Gaussian's sigma), from the number
// of darts: few darts need much to show a pattern, many show it themselves and would be
// blurred into one glow (759 darts at 10 mm covered the whole board). Like a kernel
// density estimate's rule of thumb, it shrinks with the fifth root of the darts: 12 mm
// up to 30 darts, 7.7 mm at 300, 6.4 mm at 750, 4 mm (one cell) from 7800.
func heatSpread(darts int) float64 {
	return min(max(24*math.Pow(float64(max(darts, 1)), -0.2), heatPixel), 12)
}

// heatImage is the heatmap as a PNG to lay over the board (see heatPos).
func heatImage(h stats.Heat) templ.SafeURL {
	n := 2 * heatHalf
	density := make([]float64, n*n)
	for _, c := range h.Cells {
		x := heatHalf + int(math.Floor(c.X/heatPixel))
		y := heatHalf - 1 - int(math.Floor(c.Y/heatPixel))
		if x >= 0 && y >= 0 && x < n && y < n {
			density[y*n+x] += float64(c.Count)
		}
	}
	density = blurGauss(density, n, heatSpread(h.Darts)/heatPixel)

	peak := 0.0
	for _, d := range density {
		peak = max(peak, d)
	}
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	if peak > 0 {
		for i, d := range density {
			x, y := i%n, i/n
			// Only on the board: the spread of a miss beside it is cut at its edge.
			dx, dy := (float64(x-heatHalf)+0.5)*heatPixel, (float64(y-heatHalf)+0.5)*heatPixel
			if math.Hypot(dx, dy) <= heatBoardRadius {
				img.SetNRGBA(x, y, heatColor(d/peak))
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // writing to memory doesn't fail
	return templ.SafeURL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
}

// heatPos and heatSize place the heatmap's image in the board's drawing, its middle on
// the bull.
var (
	heatSide = float64(2*heatHalf) * heatPixel
	heatPos  = strconv.FormatFloat(boardCenter-heatSide/2, 'f', 1, 64)
	heatSize = strconv.FormatFloat(heatSide, 'f', 1, 64)
)

// blurGauss smooths a square grid with a Gaussian of sigma pixels, once along the rows
// and once along the columns.
func blurGauss(src []float64, n int, sigma float64) []float64 {
	r := int(math.Ceil(3 * sigma))
	kernel := make([]float64, 2*r+1)
	for i := range kernel {
		d := float64(i - r)
		kernel[i] = math.Exp(-d * d / (2 * sigma * sigma))
	}
	pass := func(src []float64, step func(i, k int) (int, bool)) []float64 {
		dst := make([]float64, len(src))
		for i, v := range src {
			if v == 0 {
				continue
			}
			for k, w := range kernel {
				if j, ok := step(i, k-r); ok {
					dst[j] += v * w
				}
			}
		}
		return dst
	}
	rows := pass(src, func(i, d int) (int, bool) {
		x := i%n + d
		return i + d, x >= 0 && x < n
	})
	return pass(rows, func(i, d int) (int, bool) {
		y := i/n + d
		return i + d*n, y >= 0 && y < n
	})
}

// heatStops is the color scale of the heatmap, from no darts (transparent) over the
// board's red and amber to cream where most land.
var heatStops = []struct {
	at   float64
	rgba color.NRGBA
}{
	{0, color.NRGBA{208, 49, 58, 0}},
	{0.15, color.NRGBA{208, 49, 58, 110}},
	{0.45, color.NRGBA{240, 110, 40, 180}},
	{0.75, color.NRGBA{251, 191, 36, 220}},
	{1, color.NRGBA{255, 244, 214, 240}},
}

// heatColor returns the color for a density of 0 to 1 (of the highest).
func heatColor(t float64) color.NRGBA {
	for i := 1; i < len(heatStops); i++ {
		a, b := heatStops[i-1], heatStops[i]
		if t <= b.at {
			f := (t - a.at) / (b.at - a.at)
			mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + f*(float64(y)-float64(x)))) }
			return color.NRGBA{mix(a.rgba.R, b.rgba.R), mix(a.rgba.G, b.rgba.G), mix(a.rgba.B, b.rgba.B), mix(a.rgba.A, b.rgba.A)}
		}
	}
	return heatStops[len(heatStops)-1].rgba
}
