package dartboard

import (
	"errors"
	"fmt"
	"strconv"
)

// BullNumber is the number of the bull and the bullseye: the bull scores 25, the
// bullseye 50.
const BullNumber = 25

var ErrInvalid = errors.New("invalid dart")

// Hit is where a dart hit the board: the field (number and segment) and, if the cameras
// saw it, the position. It is what darts-o-cam reports to the server for every dart, so it
// has no more than that (the server's log of a game has more, see internal/server/domain/throw).
type Hit struct {
	// Number is the field's number, 25 for the bull and the bullseye; for a miss the
	// number it went beside (the lines between the numbers drawn on, from the cameras'
	// position; 0 if unknown).
	Number  uint8   `json:"number"`
	Segment Segment `json:"segment,omitempty"`
	// X and Y are where the cameras saw the dart, in mm on the board (origin in the
	// bull, Y towards the 20); nil for darts entered by hand. Correcting a dart changes
	// its field, not its position, so a field that doesn't match the position at X/Y
	// shows how far off the cameras were.
	X *float64 `json:"x,omitempty"`
	Y *float64 `json:"y,omitempty"`
}

// Validate checks that the dart hit a field that exists on the board.
func (h Hit) Validate() error {
	switch {
	case h.Segment == NoSegment || !h.Segment.valid():
		return fmt.Errorf("%w: no segment", ErrInvalid)
	case h.Segment == Miss:
		if h.Number > 20 {
			return fmt.Errorf("%w: a miss beside number %d", ErrInvalid, h.Number)
		}
	case h.Segment.IsBull():
		if h.Number != BullNumber {
			return fmt.Errorf("%w: the %s is number %d, not %d", ErrInvalid, h.Segment, BullNumber, h.Number)
		}
	case h.Number < 1 || h.Number > 20:
		return fmt.Errorf("%w: number %d does not exist", ErrInvalid, h.Number)
	}
	return nil
}

// Score returns the points of the dart, 0 for a miss.
func (h Hit) Score() uint64 {
	return uint64(h.Number) * uint64(h.Segment.Multiplier())
}

// String returns the dart as players read it: 20, D20, T20, Bull, Bullseye, M7 (a miss
// beside the 7), Miss (a miss, unknown where). The web interfaces translate it.
func (h Hit) String() string {
	n := strconv.Itoa(int(h.Number))
	switch h.Segment {
	case Miss:
		if h.Number > 0 {
			return "M" + n
		}
		return "Miss"
	case Bull:
		return "Bull"
	case Bullseye:
		return "Bullseye"
	case Double:
		return "D" + n
	case Triple:
		return "T" + n
	}
	return n
}
