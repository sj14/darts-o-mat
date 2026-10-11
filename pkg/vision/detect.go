package vision

import (
	"errors"
	"image"
	"image/draw"
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// DetectOptions tune the dart detection. The zero value is not useful, see DefaultDetectOptions.
type DetectOptions struct {
	// DiffThreshold is the difference (0-255) in any color channel above which a pixel counts as changed.
	DiffThreshold uint8
	// WeakThreshold is a lower difference that only counts for pixels connected to
	// pixels above DiffThreshold. It keeps the faint, thin point attached to the
	// barrel without picking up noise elsewhere.
	WeakThreshold uint8
	// BlurRadius smooths the images before comparing them against noise; 1 means 3x3 pixels.
	BlurRadius int
	// MinPixels is the minimum size of the changed area to be a dart.
	MinPixels int
	// MinSpeck is the size below which separate changed specks are ignored as noise.
	MinSpeck int
	// MinElongation is the minimum ratio of the length to the width of a dart.
	MinElongation float64
	// GapRadius bridges gaps of up to twice this many pixels when grouping changed pixels into
	// pieces: a barrel in front of black fields often only shows as a dashed line.
	GapRadius int
}

var DefaultDetectOptions = DetectOptions{DiffThreshold: 25, WeakThreshold: 12, BlurRadius: 2, MinPixels: 150, MinSpeck: 15, MinElongation: 4, GapRadius: 3}

var (
	ErrNoDart       = errors.New("no dart found")
	ErrNotElongated = errors.New("changed area is not shaped like a dart")
)

// Detection is a dart found in one camera image.
type Detection struct {
	Tip    dartboard.Point // in image pixels
	Pixels int             // size of the changed area
	Axis   dartboard.Point // unit vector from the tip towards the flight, in image pixels
}

// ToRGBA converts an image to RGBA with its bounds starting at 0,0.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Bounds().Min == (image.Point{}) {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

// Mask marks the changed pixels of an image.
type Mask struct {
	W, H int
	Pix  []bool
}

func (m *Mask) at(x, y int) bool {
	return x >= 0 && y >= 0 && x < m.W && y < m.H && m.Pix[y*m.W+x]
}

// Count returns the number of changed pixels.
func (m *Mask) Count() int {
	n := 0
	for _, p := range m.Pix {
		if p {
			n++
		}
	}
	return n
}

// Diff returns the pixels where any color channel differs by more than threshold,
// plus the pixels above weak that are connected to them (hysteresis).
// Colors matter: a silver dart on the red or green bull has about the same brightness.
// Both images are blurred first, so that noise and tiny camera shakes don't count.
// Thin structures like the dart's point are kept, so there is no morphological cleanup.
func Diff(before, after *image.RGBA, threshold, weak uint8, blurRadius int) *Mask {
	b, a := blur(before, blurRadius), blur(after, blurRadius)
	w, h := b.Bounds().Dx(), b.Bounds().Dy()

	diff := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*a.Stride + x*4
			var d int
			for c := 0; c < 3; c++ {
				d = max(d, abs(int(a.Pix[i+c])-int(b.Pix[i+c])))
			}
			diff[y*w+x] = uint8(min(d, 255))
		}
	}

	m := &Mask{W: w, H: h, Pix: make([]bool, w*h)}
	var queue []int
	for i, d := range diff {
		if d > threshold {
			m.Pix[i] = true
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		i := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		x, y := i%w, i/w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || yy < 0 || xx >= w || yy >= h {
					continue
				}
				j := yy*w + xx
				if !m.Pix[j] && diff[j] > weak {
					m.Pix[j] = true
					queue = append(queue, j)
				}
			}
		}
	}
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// DetectDart finds the dart that appeared between before and after and returns its tip.
// All changed areas except tiny specks count as the dart: it often falls apart into
// pieces where it crosses board areas of similar brightness.
func DetectDart(before, after *image.RGBA, opts DetectOptions) (Detection, error) {
	seed, area := dartArea(before, after, opts)
	if len(area) < opts.MinPixels {
		return Detection{}, ErrNoDart
	}
	if seed == nil {
		return Detection{}, ErrNotElongated
	}
	return tipOf(seed, area)
}

// ChangedArea returns the changed pixels that DetectDart looks at, e.g. to debug a detection.
func ChangedArea(before, after *image.RGBA, opts DetectOptions) []image.Point {
	_, area := dartArea(before, after, opts)
	return area
}

// dartArea finds the pixels of the new dart. The changed area often falls apart: a dark
// shaft on a black field doesn't show, so flight and barrel become separate pieces, and
// the thin point may break off. The seed is the largest elongated piece (barrel and point;
// for pieces that include the flight, their lower half must be elongated). Pieces on the
// seed's axis, like the flight above, belong to the dart; specks elsewhere don't.
func dartArea(before, after *image.RGBA, opts DetectOptions) (seed, area []image.Point) {
	mask := Diff(before, after, opts.DiffThreshold, opts.WeakThreshold, opts.BlurRadius)
	var comps [][]image.Point
	for _, c := range mask.componentsBridging(opts.GapRadius) {
		if len(c) >= opts.MinSpeck {
			comps = append(comps, c)
		}
	}
	slices.SortFunc(comps, func(a, b []image.Point) int { return len(b) - len(a) })

	seedIndex := -1
	for i, c := range comps {
		if lower := lowerHalf(c); len(lower) >= 10 {
			if _, _, e := principalAxis(lower); e >= opts.MinElongation {
				seedIndex = i
				break
			}
		}
	}
	if seedIndex < 0 {
		// Nothing dart-like; return everything for debugging.
		for _, c := range comps {
			area = append(area, c...)
		}
		return nil, area
	}

	seed = comps[seedIndex]
	center, axis, _ := principalAxis(lowerHalf(seed))
	for _, c := range comps {
		cc, _, _ := principalAxis(c)
		// Distance of the piece's center from the axis line.
		if math.Abs((cc.X-center.X)*axis.Y-(cc.Y-center.Y)*axis.X) <= 12 {
			area = append(area, c...)
		}
	}
	return seed, area
}

// lowerHalf returns the pixels in the lower half of the area's height: for a dart that's
// the barrel and the point, without the wide flight, which would skew the axis.
func lowerHalf(area []image.Point) []image.Point {
	minY, maxY := area[0].Y, area[0].Y
	for _, p := range area {
		minY, maxY = min(minY, p.Y), max(maxY, p.Y)
	}
	mid := (minY + maxY) / 2
	var lower []image.Point
	for _, p := range area {
		if p.Y >= mid {
			lower = append(lower, p)
		}
	}
	return lower
}

// principalAxis returns the center of the points, the unit vector of their main
// direction and how elongated they are (length to width, +Inf for a line).
func principalAxis(pts []image.Point) (center, axis dartboard.Point, elongation float64) {
	n := float64(len(pts))
	for _, p := range pts {
		center.X += float64(p.X)
		center.Y += float64(p.Y)
	}
	center.X /= n
	center.Y /= n

	var sxx, syy, sxy float64
	for _, p := range pts {
		dx, dy := float64(p.X)-center.X, float64(p.Y)-center.Y
		sxx += dx * dx
		syy += dy * dy
		sxy += dx * dy
	}
	sxx /= n
	syy /= n
	sxy /= n

	// Eigenvalues and the eigenvector of the larger one.
	tr, det := sxx+syy, sxx*syy-sxy*sxy
	disc := math.Sqrt(math.Max(tr*tr/4-det, 0))
	l1, l2 := tr/2+disc, tr/2-disc
	axis = dartboard.Point{X: sxy, Y: l1 - sxx}
	if math.Abs(sxy) < 1e-9 {
		axis = dartboard.Point{X: 1}
		if syy > sxx {
			axis = dartboard.Point{Y: 1}
		}
	}
	norm := math.Hypot(axis.X, axis.Y)
	axis.X /= norm
	axis.Y /= norm

	elongation = math.Inf(1)
	if l2 > 0 {
		elongation = math.Sqrt(l1 / l2)
	}
	return center, axis, elongation
}

// tipOf finds the tip of the dart: the lower end of the area along the seed's axis.
// The cameras look across the board from its rim, so a dart sticking out of the
// board points up in the image.
func tipOf(seed, area []image.Point) (Detection, error) {
	_, axis, _ := principalAxis(lowerHalf(seed))
	// Let the axis point down, towards the tip.
	if axis.Y < 0 || (axis.Y == 0 && axis.X < 0) {
		axis.X, axis.Y = -axis.X, -axis.Y
	}

	ts := make([]float64, len(area))
	for i, p := range area {
		ts[i] = float64(p.X)*axis.X + float64(p.Y)*axis.Y
	}
	sorted := slices.Clone(ts)
	slices.Sort(sorted)
	// Skip the 3 most extreme pixels, so single stray pixels don't move the tip.
	// (A percentile would cut off too much: the point is only about 2 px wide.)
	tipT := sorted[max(len(sorted)-4, 0)]

	// The tip is the center of the pixels within 3 px of the tip end. (Moving it along the
	// axis to the very end was tried on real throws in 2026-10 and didn't help.)
	var tx, ty float64
	var count int
	for i, p := range area {
		if math.Abs(ts[i]-tipT) <= 3 {
			tx += float64(p.X)
			ty += float64(p.Y)
			count++
		}
	}
	if count == 0 {
		return Detection{}, ErrNoDart
	}
	return Detection{
		Tip:    dartboard.Point{X: tx / float64(count), Y: ty / float64(count)},
		Pixels: len(area),
		Axis:   dartboard.Point{X: -axis.X, Y: -axis.Y},
	}, nil
}

// FuseTips combines the dart positions on the board seen by several cameras.
// It takes the median of each coordinate, so a single camera that is far off
// (e.g. because the dart hides its tip) doesn't move the result.
func FuseTips(points []dartboard.Point) (dartboard.Point, error) {
	if len(points) == 0 {
		return dartboard.Point{}, ErrNoDart
	}
	xs := make([]float64, len(points))
	ys := make([]float64, len(points))
	for i, p := range points {
		xs[i], ys[i] = p.X, p.Y
	}
	return dartboard.Point{X: median(xs), Y: median(ys)}, nil
}

func median(v []float64) float64 {
	slices.Sort(v)
	if len(v)%2 == 1 {
		return v[len(v)/2]
	}
	return (v[len(v)/2-1] + v[len(v)/2]) / 2
}

// blur averages each pixel with its (2r+1)x(2r+1) neighborhood, per color channel.
func blur(src *image.RGBA, r int) *image.RGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	tmp := make([]int, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				sum, n := 0, 0
				for dx := -r; dx <= r; dx++ {
					if xx := x + dx; xx >= 0 && xx < w {
						sum += int(src.Pix[y*src.Stride+xx*4+c])
						n++
					}
				}
				tmp[(y*w+x)*4+c] = sum / n
			}
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				sum, n := 0, 0
				for dy := -r; dy <= r; dy++ {
					if yy := y + dy; yy >= 0 && yy < h {
						sum += tmp[(yy*w+x)*4+c]
						n++
					}
				}
				out.Pix[y*out.Stride+x*4+c] = uint8(sum / n)
			}
			out.Pix[y*out.Stride+x*4+3] = 255
		}
	}
	return out
}

// componentsBridging returns the connected areas of the mask like components, but
// pixels up to 2r apart count as connected. Only the mask's own pixels are returned.
func (m *Mask) componentsBridging(r int) [][]image.Point {
	if r <= 0 {
		return m.components()
	}
	grown := m.dilate(r)
	var result [][]image.Point
	for _, c := range grown.components() {
		var own []image.Point
		for _, p := range c {
			if m.Pix[p.Y*m.W+p.X] {
				own = append(own, p)
			}
		}
		if len(own) > 0 {
			result = append(result, own)
		}
	}
	return result
}

// dilate grows every set pixel to a (2r+1)x(2r+1) square, separably for speed.
func (m *Mask) dilate(r int) *Mask {
	tmp := make([]bool, len(m.Pix))
	for y := 0; y < m.H; y++ {
		run := -1 // last set pixel in this row
		for x := 0; x < m.W+r; x++ {
			if x < m.W && m.Pix[y*m.W+x] {
				run = x
			}
			if xx := x - r; xx >= 0 && xx < m.W && run >= 0 && x-run <= 2*r {
				tmp[y*m.W+xx] = true
			}
		}
	}
	out := &Mask{W: m.W, H: m.H, Pix: make([]bool, len(m.Pix))}
	for x := 0; x < m.W; x++ {
		run := -1
		for y := 0; y < m.H+r; y++ {
			if y < m.H && tmp[y*m.W+x] {
				run = y
			}
			if yy := y - r; yy >= 0 && yy < m.H && run >= 0 && y-run <= 2*r {
				out.Pix[yy*m.W+x] = true
			}
		}
	}
	return out
}

// components returns the connected areas of the mask.
func (m *Mask) components() [][]image.Point {
	seen := make([]bool, len(m.Pix))
	var result [][]image.Point
	var queue []image.Point
	for i, set := range m.Pix {
		if !set || seen[i] {
			continue
		}
		var comp []image.Point
		queue = append(queue[:0], image.Point{X: i % m.W, Y: i / m.W})
		seen[i] = true
		for len(queue) > 0 {
			p := queue[len(queue)-1]
			queue = queue[:len(queue)-1]
			comp = append(comp, p)
			for _, d := range [8]image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
				q := p.Add(d)
				j := q.Y*m.W + q.X
				if m.at(q.X, q.Y) && !seen[j] {
					seen[j] = true
					queue = append(queue, q)
				}
			}
		}
		result = append(result, comp)
	}
	return result
}
