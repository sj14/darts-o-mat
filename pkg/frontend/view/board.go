package view

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/service"
)

// The board is drawn in mm like the dartboard package, with y flipped for SVG and the
// bull in the middle of the drawing. (Not with a negative viewBox origin: some
// renderers, e.g. moli, don't move text by it.)
const (
	boardSize    = 456.0
	boardCenter  = boardSize / 2
	boardViewBox = "0 0 456 456"
)

// boardShape is a field of the drawn board.
type boardShape struct {
	Path, Fill string
}

// boardNumber is a number around the board.
type boardNumber struct {
	X, Y string
	Text string
}

// boardDart is a dart marked on the board.
type boardDart struct {
	X, Y   string
	Label  string // 1 to 3, the dart of the turn
	Fill   string // color of the marker
	Latest bool   // the dart just thrown, animated
	Faded  bool   // a dart of the previous turn
}

// Colors of the board: like a real one, a bit darker to fit the page.
const (
	boardBlack = "#17181c"
	boardCream = "#e9dcc0"
	boardRed   = "#d0313a"
	boardGreen = "#1f9a5c"
	boardWire  = "#8b8f98"
	// boardTarget marks the field to hit next in around the clock (amber like the other highlights).
	boardTarget = "#fbbf24"
)

var boardShapes, boardNumbers = drawBoard()

// svgX and svgY turn a board position into drawing coordinates.
func svgX(p dartboard.Point) string { return strconv.FormatFloat(boardCenter+p.X, 'f', 2, 64) }
func svgY(p dartboard.Point) string { return strconv.FormatFloat(boardCenter-p.Y, 'f', 2, 64) }

func svgPoint(p dartboard.Point) string {
	return svgX(p) + " " + svgY(p)
}

// ringPath is the outline of a segment between two radii and two angles (clockwise from the top).
func ringPath(r1, r2, a1, a2 float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "M%s", svgPoint(dartboard.Polar(r2, a1)))
	fmt.Fprintf(&b, "A%g %g 0 0 1 %s", r2, r2, svgPoint(dartboard.Polar(r2, a2)))
	fmt.Fprintf(&b, "L%s", svgPoint(dartboard.Polar(r1, a2)))
	fmt.Fprintf(&b, "A%g %g 0 0 0 %s", r1, r1, svgPoint(dartboard.Polar(r1, a1)))
	b.WriteString("Z")
	return b.String()
}

func drawBoard() ([]boardShape, []boardNumber) {
	var shapes []boardShape
	var numbers []boardNumber
	for i, n := range dartboard.Segments {
		center := float64(i) * dartboard.SegmentAngle
		a1, a2 := center-dartboard.SegmentAngle/2, center+dartboard.SegmentAngle/2
		single, ring := boardBlack, boardRed
		if i%2 == 1 {
			single, ring = boardCream, boardGreen
		}
		shapes = append(shapes,
			boardShape{ringPath(dartboard.BullRadius, dartboard.TripleInnerRadius, a1, a2), single},
			boardShape{ringPath(dartboard.TripleInnerRadius, dartboard.TripleOuterRadius, a1, a2), ring},
			boardShape{ringPath(dartboard.TripleOuterRadius, dartboard.DoubleInnerRadius, a1, a2), single},
			boardShape{ringPath(dartboard.DoubleInnerRadius, dartboard.DoubleOuterRadius, a1, a2), ring},
		)
		p := dartboard.Polar(197, center)
		numbers = append(numbers, boardNumber{X: svgX(p), Y: svgY(p), Text: strconv.Itoa(int(n))})
	}
	return shapes, numbers
}

// targetPath outlines the field to hit next in around the clock: the whole segment of
// the number, since single, double and triple all count, or the bull.
func targetPath(target uint8) string {
	if target == throw.BullNumber {
		r := dartboard.BullRadius
		return fmt.Sprintf("M%g %gA%g %g 0 1 0 %g %gA%g %g 0 1 0 %g %gZ", boardCenter-r, boardCenter, r, r, boardCenter+r, boardCenter, r, r, boardCenter-r, boardCenter)
	}
	center := float64(dartboard.SegmentIndex(target)) * dartboard.SegmentAngle
	return ringPath(dartboard.BullRadius, dartboard.DoubleOuterRadius, center-dartboard.SegmentAngle/2, center+dartboard.SegmentAngle/2)
}

// dimPath covers the playing area except the target field (with fill-rule evenodd).
func dimPath(target uint8) string {
	r := float64(dartboard.DoubleOuterRadius)
	circle := fmt.Sprintf("M%g %gA%g %g 0 1 0 %g %gA%g %g 0 1 0 %g %gZ", boardCenter-r, boardCenter, r, r, boardCenter+r, boardCenter, r, r, boardCenter-r, boardCenter)
	return circle + targetPath(target)
}

// boardTargetOf returns the field to highlight on the board, 0 for none: the current
// player's target in around the clock, not while bulling off.
func boardTargetOf(v service.GameView, p progress) uint8 {
	if !p.running() || p.BullingOff {
		return 0
	}
	return currentTarget(v)
}

// fieldCenter is where a dart entered by hand is drawn: in the middle of its field.
// Several darts in the same field are moved apart a little.
func fieldCenter(d throw.Dart, nth int) (dartboard.Point, bool) {
	shift := float64(nth) * 5
	switch d.Segment {
	case throw.Miss, throw.NoSegment:
		return dartboard.Point{}, false
	case throw.Bullseye:
		return dartboard.Polar(shift/2, 90*float64(nth)), true
	case throw.Bull:
		return dartboard.Polar(11, 45+90*float64(nth)), true
	}
	i := dartboard.SegmentIndex(d.Number)
	if i < 0 {
		return dartboard.Point{}, false
	}
	radius := map[throw.Segment]float64{
		throw.Inner:  (dartboard.BullRadius + dartboard.TripleInnerRadius) / 2,
		throw.Single: (dartboard.TripleOuterRadius + dartboard.DoubleInnerRadius) / 2,
		throw.Outer:  (dartboard.TripleOuterRadius + dartboard.DoubleInnerRadius) / 2,
		throw.Double: (dartboard.DoubleInnerRadius + dartboard.DoubleOuterRadius) / 2,
		throw.Triple: (dartboard.TripleInnerRadius + dartboard.TripleOuterRadius) / 2,
	}[d.Segment]
	return dartboard.Polar(radius, float64(i)*dartboard.SegmentAngle+shift-5), true
}

// boardDarts returns the darts to mark: those of the current turn, or the previous
// turn's (faded) right after a turn passed on. While bulling off, and until the first
// dart of the game, the bull-off darts (the ties' earlier rounds faded).
func boardDarts(v service.GameView, p progress) []boardDart {
	if bull := p.BullOff; len(bull) > 0 && (p.BullingOff || onlyTakeoutsAfter(v, bull[len(bull)-1].Entry)) {
		round := bull[len(bull)-1].Round
		var darts []throw.Dart
		var faded []bool
		for _, b := range bull {
			darts, faded = append(darts, v.Darts[b.Entry]), append(faded, b.Round != round)
		}
		return markDarts(v, darts, faded)
	}
	darts, faded := turnDarts(v, p), false
	if len(darts) == 0 {
		darts, faded = previousTurn(v), true
	}
	return markDarts(v, darts, slices.Repeat([]bool{faded}, len(darts)))
}

// onlyTakeoutsAfter reports whether the log has no dart after entry (takeouts and players
// leaving aside).
func onlyTakeoutsAfter(v service.GameView, entry int) bool {
	for _, d := range v.Darts[entry+1:] {
		if d.IsDart() {
			return false
		}
	}
	return true
}

// markDarts places darts on the board, numbered in order: at the position the cameras
// saw, or in the middle of their field.
func markDarts(v service.GameView, darts []throw.Dart, faded []bool) []boardDart {
	var marks []boardDart
	for i, d := range darts {
		pos, ok := dartboard.Point{}, false
		if d.X != nil && d.Y != nil {
			pos, ok = dartboard.Point{X: *d.X, Y: *d.Y}, true
		} else {
			nth := 0
			for _, e := range darts[:i] {
				if e.Number == d.Number && e.Segment == d.Segment {
					nth++
				}
			}
			pos, ok = fieldCenter(d, nth)
		}
		if !ok {
			continue
		}
		fill := "#f5b70a"
		switch dartClass(v, d) {
		case "hit":
			fill = "#34d399"
		case "miss":
			fill = "#f87171"
		}
		marks = append(marks, boardDart{
			X:      svgX(pos),
			Y:      svgY(pos),
			Label:  strconv.Itoa(i + 1),
			Fill:   fill,
			Latest: !faded[i] && isLatest(v, d),
			Faded:  faded[i],
		})
	}
	return marks
}

// previousTurn returns the darts of the turn before the current one.
func previousTurn(v service.GameView) []throw.Dart {
	end := len(v.Darts)
	for end > 0 && !v.Darts[end-1].IsDart() {
		end--
	}
	if end == 0 {
		return nil
	}
	start := end - 1
	for start > 0 && v.Darts[start-1].IsDart() && v.Darts[start-1].PlayerID == v.Darts[end-1].PlayerID && end-start < turn.DartsPerTurn {
		start--
	}
	return v.Darts[start:end]
}
