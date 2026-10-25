package vision

import "image"

// matchExposure returns after with the brightness of before: each color channel mapped so
// that its histogram matches before's. Where the exposure can't be locked (Linux,
// Windows), a camera readjusts it when someone stands in front of the board, and the
// frame after a takeout is up to 27 levels brighter than the one before: up to 885,595 of
// 921,600 px changed, and the takeout vote counted a camera like that as a throw
// (2026-10-25, event 9, cam1). A camera's exposure changes its tone curve, not just a
// factor: scaling by the median ratio left 90,000 to 230,000 px of such changes and lost
// a thrown dart, matching the histograms leaves about the darts pulled out (725,451 →
// 45,143 px) and moves the tips of thrown darts by at most 1 px. A new dart changes the
// histograms too little to matter. About 15 ms per 1280×720 frame, once per camera when
// the board is still after a change.
//
// Only if most of the image changed its brightness (see minExposureShift); otherwise it
// returns after as it is: matching also maps away what is new in the image, entirely on a
// uniform one, and it would change the detection where there is nothing to correct.
func matchExposure(before, after *image.RGBA) *image.RGBA {
	if s := medianShift(before, after); s > -minExposureShift && s < minExposureShift {
		return after
	}
	var hb, ha [3][256]int
	for i := 0; i < len(after.Pix); i += 4 {
		for c := range 3 {
			hb[c][before.Pix[i+c]]++
			ha[c][after.Pix[i+c]]++
		}
	}
	// Each level of after becomes the lowest level of before whose cumulative count
	// reaches its own.
	var lut [3][256]uint8
	for c := range 3 {
		var sb, sa, j int
		sb = hb[c][0]
		for v := range 256 {
			sa += ha[c][v]
			for j < 255 && sb < sa {
				j++
				sb += hb[c][j]
			}
			lut[c][v] = uint8(j)
		}
	}
	out := &image.RGBA{Pix: make([]uint8, len(after.Pix)), Stride: after.Stride, Rect: after.Rect}
	for i := 0; i < len(after.Pix); i += 4 {
		out.Pix[i] = lut[0][after.Pix[i]]
		out.Pix[i+1] = lut[1][after.Pix[i+1]]
		out.Pix[i+2] = lut[2][after.Pix[i+2]]
		out.Pix[i+3] = after.Pix[i+3]
	}
	return out
}

// minExposureShift is the median change in brightness from which a camera's exposure
// counts as changed. On 780 frames of 260 saved events (2026-10-25) the median was 0 for
// 675 and ±1 for 16, the others moved from 2 up to 30 levels, nearly all of them at a
// takeout.
const minExposureShift = 2

// medianShift returns the median change in brightness (mean of the color channels) from
// before to after, over every 9th pixel.
func medianShift(before, after *image.RGBA) int {
	var hist [511]int
	n := 0
	for i := 0; i < len(after.Pix); i += 4 * 9 {
		d := (int(after.Pix[i]) + int(after.Pix[i+1]) + int(after.Pix[i+2]) - int(before.Pix[i]) - int(before.Pix[i+1]) - int(before.Pix[i+2])) / 3
		hist[d+255]++
		n++
	}
	sum := 0
	for k, c := range hist {
		if sum += c; 2*sum >= n {
			return k - 255
		}
	}
	return 0
}
