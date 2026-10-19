package dartboard

import (
	"database/sql/driver"
	"fmt"
)

// Segment is the part of the board a dart hit. It decides the multiplier of the dart's
// number (Multiplier); games that need to know more, e.g. the inner or outer single
// field, read the segment itself.
//
// In the database it is stored as its number (user's choice, 2026-10-14), so the order
// of the constants must never change; JSON and forms use the names.
type Segment uint8

const (
	// NoSegment is the zero value: not given (e.g. a takeout, or a dart entered without
	// a segment, which counts as Single).
	NoSegment Segment = iota
	Miss              // off the scoring area
	Single            // a single field, inner or outer unknown (entered by hand)
	Inner             // the single field between the bull and the triple ring
	Outer             // the single field between the triple and the double ring
	Double
	Triple
	Bull     // the outer bull, 25
	Bullseye // the inner bull, 50
)

var segmentNames = [...]string{
	NoSegment: "",
	Miss:      "miss",
	Single:    "single",
	Inner:     "inner",
	Outer:     "outer",
	Double:    "double",
	Triple:    "triple",
	Bull:      "bull",
	Bullseye:  "bullseye",
}

// Multiplier returns what the segment multiplies the number with: 0 for a miss, 1 for
// the single fields and the bull, 2 for the double and the bullseye, 3 for the triple.
func (s Segment) Multiplier() int {
	switch s {
	case Single, Inner, Outer, Bull:
		return 1
	case Double, Bullseye:
		return 2
	case Triple:
		return 3
	}
	return 0
}

// IsDouble reports whether the segment is a double: the double ring or the bullseye
// (both finish a double-out game).
func (s Segment) IsDouble() bool {
	return s == Double || s == Bullseye
}

// IsSingle reports whether the segment is a single field, inner, outer or unknown.
func (s Segment) IsSingle() bool {
	return s == Single || s == Inner || s == Outer
}

// IsBull reports whether the segment is the bull or the bullseye.
func (s Segment) IsBull() bool {
	return s == Bull || s == Bullseye
}

func (s Segment) valid() bool {
	return int(s) < len(segmentNames)
}

func (s Segment) String() string {
	if !s.valid() {
		return fmt.Sprintf("Segment(%d)", uint8(s))
	}
	return segmentNames[s]
}

// ParseSegment returns the segment of a name (see String).
func ParseSegment(name string) (Segment, error) {
	for s, n := range segmentNames {
		if n == name {
			return Segment(s), nil
		}
	}
	return NoSegment, fmt.Errorf("%w: unknown segment %q", ErrInvalid, name)
}

func (s Segment) MarshalText() ([]byte, error) {
	if !s.valid() {
		return nil, fmt.Errorf("%w: segment %d", ErrInvalid, uint8(s))
	}
	return []byte(s.String()), nil
}

func (s *Segment) UnmarshalText(text []byte) error {
	seg, err := ParseSegment(string(text))
	if err != nil {
		return err
	}
	*s = seg
	return nil
}

// Value stores the segment as its number.
func (s Segment) Value() (driver.Value, error) {
	return int64(s), nil
}

func (s *Segment) Scan(src any) error {
	n, ok := src.(int64)
	if !ok || n < 0 || !Segment(n).valid() {
		return fmt.Errorf("%w: segment %v", ErrInvalid, src)
	}
	*s = Segment(n)
	return nil
}
