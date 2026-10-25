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
// Gaussian, so where many land glows and single darts stay faint. Squares of 8 mm hold
// one or two darts each and showed as dots all over the board.
const (
	// heatPixel is the side of a pixel of the heatmap's image in mm: half a cell of
	// stats.Heat, so each cell's middle is a pixel's middle.
	heatPixel = stats.HeatCellSize / 2
	// heatSigma is how far a dart is spread out (mm): about a triple ring and a half, so
	// a group in the T20 shows as one spot, but the T20 apart from the S20 below it.
	heatSigma = 10.0
)

// heatPixels is the side of the heatmap's image: up to stats.HeatReach on each side,
// with pixel heatHalf on the bull.
var (
	heatHalf   = int(math.Ceil(stats.HeatReach / heatPixel))
	heatPixels = 2*heatHalf + 1
)

// heatImage is the heatmap as a PNG to lay over the board (see heatPos).
func heatImage(h stats.Heat) templ.SafeURL {
	density := make([]float64, heatPixels*heatPixels)
	for _, c := range h.Cells {
		x, y := heatHalf+int(math.Round(c.X/heatPixel)), heatHalf-int(math.Round(c.Y/heatPixel))
		if x >= 0 && y >= 0 && x < heatPixels && y < heatPixels {
			density[y*heatPixels+x] += float64(c.Count)
		}
	}
	density = blurGauss(density, heatPixels, heatSigma/heatPixel)

	peak := 0.0
	for _, d := range density {
		peak = max(peak, d)
	}
	img := image.NewNRGBA(image.Rect(0, 0, heatPixels, heatPixels))
	// By the square root of the share of the highest, so that a second target (e.g. the
	// T19 next to a T20 with four times as many darts) still shows.
	if peak > 0 {
		for i, d := range density {
			img.SetNRGBA(i%heatPixels, i/heatPixels, heatColor(math.Sqrt(d/peak)))
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img) // writing to memory doesn't fail
	return templ.SafeURL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()))
}

// heatPos and heatSize place the heatmap's image in the board's drawing: its middle
// pixel on the bull.
var (
	heatSide = float64(heatPixels) * heatPixel
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
