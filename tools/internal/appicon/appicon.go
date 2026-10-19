// Package appicon renders the app icon of darts-o-cam for the packaging tools
// (tools/macapp, tools/linuxpkg, tools/winpkg): the concentric rings of favicon.svg, so both stay alike.
package appicon

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"slices"
	"strconv"

	"github.com/sj14/darts-o-mat/common/static"
)

// Render renders the icon with size×size pixels.
func Render(size int) (image.Image, error) {
	rings, err := faviconRings()
	if err != nil {
		return nil, err
	}
	return renderRings(rings, size), nil
}

// WritePNG renders the icon with size×size pixels into a PNG file.
func WritePNG(path string, size int) error {
	img, err := Render(size)
	if err != nil {
		return err
	}
	return writePNG(path, img)
}

// ring is a filled circle of the favicon, in its 32×32 view box.
type ring struct {
	r   float64
	col color.RGBA
}

// faviconRings reads the concentric circles of favicon.svg, outermost first.
func faviconRings() ([]ring, error) {
	data, err := static.FS.ReadFile("favicon.svg")
	if err != nil {
		return nil, err
	}
	var svg struct {
		Circles []struct {
			R    string `xml:"r,attr"`
			Fill string `xml:"fill,attr"`
		} `xml:"circle"`
	}
	if err := xml.Unmarshal(data, &svg); err != nil {
		return nil, err
	}
	var rings []ring
	for _, c := range svg.Circles {
		r, err := strconv.ParseFloat(c.R, 64)
		if err != nil {
			return nil, fmt.Errorf("favicon.svg: radius %q: %w", c.R, err)
		}
		var col color.RGBA
		if _, err := fmt.Sscanf(c.Fill, "#%02x%02x%02x", &col.R, &col.G, &col.B); err != nil {
			return nil, fmt.Errorf("favicon.svg: color %q: %w", c.Fill, err)
		}
		col.A = 255
		rings = append(rings, ring{r, col})
	}
	return rings, nil
}

// renderRings draws the rings into a square image. Like other macOS icons, the disc
// leaves a margin of about a tenth on each side. Edges are smoothed by sampling each
// pixel 4×4 times.
func renderRings(rings []ring, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const n = 4
	center := float64(size) / 2
	scale := float64(size) * 0.8 / 32 // the favicon's 32 units fill 80 % of the icon
	for y := range size {
		for x := range size {
			var r, g, b, hits float64
			for sy := range n {
				for sx := range n {
					dx := (float64(x) + (float64(sx)+0.5)/n - center) / scale
					dy := (float64(y) + (float64(sy)+0.5)/n - center) / scale
					d := dx*dx + dy*dy
					// The innermost ring containing the sample wins.
					for _, rg := range slices.Backward(rings) {
						if d <= rg.r*rg.r {
							c := rg.col
							r, g, b, hits = r+float64(c.R), g+float64(c.G), b+float64(c.B), hits+1
							break
						}
					}
				}
			}
			if hits > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(r / hits), uint8(g / hits), uint8(b / hits), uint8(255 * hits / (n * n))})
			}
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
