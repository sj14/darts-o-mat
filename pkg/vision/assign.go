package vision

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
)

// thumbSize is the size frames are reduced to for comparing camera views.
const thumbW, thumbH = 64, 36

// thumbnail reduces an image to a small grayscale grid by averaging blocks.
func thumbnail(img image.Image) []float64 {
	rgba := ToRGBA(img)
	w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()
	thumb := make([]float64, thumbW*thumbH)
	counts := make([]int, thumbW*thumbH)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := rgba.PixOffset(x, y)
			gray := 0.299*float64(rgba.Pix[i]) + 0.587*float64(rgba.Pix[i+1]) + 0.114*float64(rgba.Pix[i+2])
			t := (y*thumbH/h)*thumbW + x*thumbW/w
			thumb[t] += gray
			counts[t]++
		}
	}
	for i := range thumb {
		if counts[i] > 0 {
			thumb[i] /= float64(counts[i])
		}
	}
	return thumb
}

// viewDistance compares two camera views; small means similar.
func viewDistance(a, b []float64) float64 {
	var sum float64
	for i := range a {
		sum += math.Abs(a[i] - b[i])
	}
	return sum / float64(len(a))
}

var ErrAmbiguousCameras = errors.New("camera views don't match the reference images")

// AssignCameras finds which frame belongs to which reference view: result[i] is the
// index of the frame for reference i, or -1 if there are fewer frames than references
// and none is left for it. Each camera of a board sees the board from its own fixed
// angle, so this works even if the cameras are plugged in differently.
// It fails if a camera's view doesn't resemble its reference at all, e.g. a laptop
// camera got picked or the camera was moved.
func AssignCameras(frames []image.Image, references []image.Image) ([]int, error) {
	ft := make([][]float64, len(frames))
	for i, f := range frames {
		ft[i] = thumbnail(f)
	}
	rt := make([][]float64, len(references))
	for i, r := range references {
		rt[i] = thumbnail(r)
	}

	// Try all assignments; boards have only a handful of cameras. As many references
	// as possible get a frame: only the surplus of references stays without one.
	skips := max(len(references)-len(frames), 0)
	best, bestCost := []int(nil), math.Inf(1)
	var try func(assign []int, used []bool, skipped int, cost float64)
	try = func(assign []int, used []bool, skipped int, cost float64) {
		if cost >= bestCost {
			return
		}
		r := len(assign)
		if r == len(references) {
			best, bestCost = slices.Clone(assign), cost
			return
		}
		for f := range frames {
			if !used[f] {
				used[f] = true
				try(append(assign, f), used, skipped, cost+viewDistance(ft[f], rt[r]))
				used[f] = false
			}
		}
		if skipped < skips {
			try(append(assign, -1), used, skipped+1, cost)
		}
	}
	try(nil, make([]bool, len(frames)), 0, 0)

	// A matching view differs mostly by darts and lighting; a different scene by much more.
	const maxDistance = 25
	for r, f := range best {
		if f < 0 {
			continue
		}
		if d := viewDistance(ft[f], rt[r]); d > maxDistance {
			return nil, fmt.Errorf("%w: camera %d differs by %.0f from reference %d", ErrAmbiguousCameras, f, d, r)
		}
	}
	return best, nil
}
