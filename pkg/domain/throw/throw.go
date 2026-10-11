package throw

import (
	"errors"
	"fmt"
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
	Number     uint8 `json:"number"`
	Multiplier uint8 `json:"multiplier"`
	Miss       bool  `json:"miss"`
	Takeout    bool  `bun:",notnull,default:false" json:"takeout,omitempty"`
}

// Validate checks that the dart hit a field that exists on the board.
// Number and Multiplier are ignored for a miss.
func (d Dart) Validate() error {
	if d.Miss {
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
