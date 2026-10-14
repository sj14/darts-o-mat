// Package dartboard describes the geometry of a standard steel dartboard
// and scores positions on it.
//
// Positions are in millimeters with the origin in the center of the bull,
// X pointing right and Y pointing up towards the 20.
package dartboard

import (
	"math"

	"github.com/sj14/darts-o-mat/pkg/domain/throw"
)

// Radii of the standard board in mm (to the middle of the wires).
const (
	BullseyeRadius    = 6.35
	BullRadius        = 15.9
	TripleInnerRadius = 99
	TripleOuterRadius = 107
	DoubleInnerRadius = 162
	DoubleOuterRadius = 170
)

// Segments lists the numbers clockwise, starting with the 20 at the top.
var Segments = [20]uint8{20, 1, 18, 4, 13, 6, 10, 15, 2, 17, 3, 19, 7, 16, 8, 11, 14, 9, 12, 5}

// SegmentAngle is the angle each number covers, in degrees.
const SegmentAngle = 360.0 / 20

type Point struct {
	X, Y float64
}

// Polar returns the point at radius mm and angle degrees, measured clockwise from the top (the 20).
func Polar(radius, angle float64) Point {
	rad := angle * math.Pi / 180
	return Point{X: radius * math.Sin(rad), Y: radius * math.Cos(rad)}
}

// Angle returns the angle of p in degrees, clockwise from the top, in [0, 360).
func (p Point) Angle() float64 {
	a := math.Atan2(p.X, p.Y) * 180 / math.Pi
	if a < 0 {
		a += 360
	}
	return a
}

func (p Point) Radius() float64 {
	return math.Hypot(p.X, p.Y)
}

// Number returns the number of the segment p lies in, ignoring the rings.
func (p Point) Number() uint8 {
	i := int(math.Floor((p.Angle()+SegmentAngle/2)/SegmentAngle)) % len(Segments)
	return Segments[i]
}

// SegmentIndex returns the index of number in Segments, -1 if it isn't on the board.
func SegmentIndex(number uint8) int {
	for i, n := range Segments {
		if n == number {
			return i
		}
	}
	return -1
}

// Score returns the dart hitting the board at p.
func Score(p Point) throw.Dart {
	r := p.Radius()
	switch {
	case r <= BullseyeRadius:
		return throw.Dart{Number: throw.Bull, Multiplier: 2}
	case r <= BullRadius:
		return throw.Dart{Number: throw.Bull, Multiplier: 1}
	case r > DoubleOuterRadius:
		return throw.Dart{Miss: true, Number: p.Number()}
	}

	d := throw.Dart{Number: p.Number(), Multiplier: 1}
	switch {
	case r >= DoubleInnerRadius:
		d.Multiplier = 2
	case r >= TripleInnerRadius && r <= TripleOuterRadius:
		d.Multiplier = 3
	}
	return d
}
