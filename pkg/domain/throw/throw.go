package throw

import (
	"errors"
	"fmt"
	"strconv"
)

// Bull is the number of the bull. A single bull scores 25, a double bull (bullseye) 50.
const Bull = 25

var ErrInvalid = errors.New("invalid dart")

// Dart is an entry of a game's log, in the order it happened: a dart thrown, or the
// darts being pulled out of the board (Takeout), which ends a turn in games played on
// a board.
type Dart struct {
	ID         int64 `bun:",pk,autoincrement" json:"id"`
	GameID     int64 `bun:",notnull" json:"-"`
	// Number is the field's number, for a miss the number it went beside (the lines
	// between the numbers drawn on, from the cameras' position; 0 if unknown).
	Number     uint8 `json:"number"`
	Multiplier uint8 `json:"multiplier"`
	Miss       bool  `json:"miss"`
	Takeout    bool  `bun:",notnull,default:false" json:"takeout,omitempty"`
	// X and Y are where the cameras saw the dart, in mm on the board (origin in the
	// bull, Y towards the 20); nil for darts entered by hand. Correcting a dart changes
	// its field, not its position, so a field that doesn't match the position at X/Y
	// shows how far off the cameras were.
	X *float64 `json:"x,omitempty"`
	Y *float64 `json:"y,omitempty"`
}

// Validate checks that the dart hit a field that exists on the board. The Multiplier is
// ignored for a miss.
func (d Dart) Validate() error {
	if d.Miss {
		if d.Number > 20 {
			return fmt.Errorf("%w: a miss beside number %d", ErrInvalid, d.Number)
		}
		return nil
	}
	if (d.Number < 1 || d.Number > 20) && d.Number != Bull {
		return fmt.Errorf("%w: number %d does not exist", ErrInvalid, d.Number)
	}
	if d.Multiplier < 1 || d.Multiplier > 3 {
		return fmt.Errorf("%w: multiplier must be 1, 2 or 3, got %d", ErrInvalid, d.Multiplier)
	}
	if d.Number == Bull && d.Multiplier == 3 {
		return fmt.Errorf("%w: there is no triple bull", ErrInvalid)
	}
	return nil
}

// Score returns the points of the dart, 0 for a miss.
func (d Dart) Score() uint64 {
	if d.Miss {
		return 0
	}
	return uint64(d.Number) * uint64(d.Multiplier)
}

// String returns the dart as players read it: 20, D20, T20, Bull, Bullseye, M7 (a miss
// beside the 7), Daneben (a miss, unknown where).
func (d Dart) String() string {
	switch {
	case d.Miss && d.Number > 0:
		return "M" + strconv.Itoa(int(d.Number))
	case d.Miss:
		return "Daneben"
	case d.Number == Bull && d.Multiplier == 2:
		return "Bullseye"
	case d.Number == Bull:
		return "Bull"
	case d.Multiplier == 2:
		return "D" + strconv.Itoa(int(d.Number))
	case d.Multiplier == 3:
		return "T" + strconv.Itoa(int(d.Number))
	}
	return strconv.Itoa(int(d.Number))
}
