package view

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
)

// Where many darts land glows most, a single dart elsewhere shows weaker, and where none
// land nothing covers the board.
func TestHeatImage(t *testing.T) {
	t.Parallel()
	var points []dartboard.Point
	for i := range 20 {
		points = append(points, dartboard.Point{X: float64(i%5-2) * 3, Y: 103 + float64(i/5-2)*3}) // a group in the T20
	}
	points = append(points, dartboard.Point{X: -100, Y: -50}) // one dart elsewhere

	url := string(heatImage(stats.NewHeat(points)))
	data, ok := strings.CutPrefix(url, "data:image/png;base64,")
	if !ok {
		t.Fatalf("not a PNG data URL: %.40s", url)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != heatPixels || b.Dy() != heatPixels {
		t.Fatalf("image %v, want %d×%d", b, heatPixels, heatPixels)
	}
	alpha := func(p dartboard.Point) uint32 {
		x, y := heatHalf+int(math.Round(p.X/heatPixel)), heatHalf-int(math.Round(p.Y/heatPixel))
		_, _, _, a := img.At(x, y).RGBA()
		return a >> 8
	}
	group, single, empty := alpha(dartboard.Point{Y: 103}), alpha(dartboard.Point{X: -100, Y: -50}), alpha(dartboard.Point{X: 100, Y: -100})
	if group < 200 {
		t.Errorf("alpha at the group %d, want it nearly opaque", group)
	}
	if single == 0 || single > group*2/3 {
		t.Errorf("alpha at the single dart %d, want weaker than the group's %d", single, group)
	}
	if empty != 0 {
		t.Errorf("alpha where no dart landed %d, want 0", empty)
	}
}
