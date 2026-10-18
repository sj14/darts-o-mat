package throw

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

// BullNumber is the number of the bull and the bullseye: the bull scores 25, the
// bullseye 50.
const BullNumber = 25

var ErrInvalid = errors.New("invalid dart")

// Dart is an entry of a game's log, in the order it happened: a dart thrown, or the
// darts being pulled out of the board (Takeout), which ends a turn in games played on
// a board.
type Dart struct {
	bun.BaseModel `bun:"table:darts"`

	ID     int64 `bun:",pk,autoincrement" json:"id"`
	GameID int64 `bun:",notnull" json:"-"`
	// Number is the field's number, 25 for the bull and the bullseye; for a miss the
	// number it went beside (the lines between the numbers drawn on, from the cameras'
	// position; 0 if unknown).
	Number  uint8   `json:"number"`
	Segment Segment `bun:",notnull" json:"segment,omitempty"`
	Takeout bool    `bun:",notnull,default:false" json:"takeout,omitempty"`
	// PlayerID is the player who threw the dart, set by the server from whose turn it was
	// (0 for takeouts); for a Leave entry the player who left. Replaying a game checks
	// that it is the player who is up.
	PlayerID int64 `bun:",notnull,default:0" json:"player_id,omitempty"`
	// Leave marks the entry of the log in which the player PlayerID left the game; the
	// others play on without them (see turn.Turn.Leave).
	Leave bool `bun:",notnull,default:false" json:"leave,omitempty"`
	// X and Y are where the cameras saw the dart, in mm on the board (origin in the
	// bull, Y towards the 20); nil for darts entered by hand. Correcting a dart changes
	// its field, not its position, so a field that doesn't match the position at X/Y
	// shows how far off the cameras were.
	X *float64 `json:"x,omitempty"`
	Y *float64 `json:"y,omitempty"`
	// CreatedAt is when the entry was made (for the cameras: when the dart was thrown).
	// A leg, set or game lasts from its first dart to its last (turn.Span).
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at,omitzero"`
}

// IsDart reports whether the entry of a game's log is a dart, and not a takeout or a
// player leaving.
func (d Dart) IsDart() bool {
	return !d.Takeout && !d.Leave
}

// Validate checks that the dart hit a field that exists on the board.
func (d Dart) Validate() error {
	switch {
	case d.Segment == NoSegment || !d.Segment.valid():
		return fmt.Errorf("%w: no segment", ErrInvalid)
	case d.Segment == Miss:
		if d.Number > 20 {
			return fmt.Errorf("%w: a miss beside number %d", ErrInvalid, d.Number)
		}
	case d.Segment.IsBull():
		if d.Number != BullNumber {
			return fmt.Errorf("%w: the %s is number %d, not %d", ErrInvalid, d.Segment, BullNumber, d.Number)
		}
	case d.Number < 1 || d.Number > 20:
		return fmt.Errorf("%w: number %d does not exist", ErrInvalid, d.Number)
	}
	return nil
}

// Score returns the points of the dart, 0 for a miss.
func (d Dart) Score() uint64 {
	return uint64(d.Number) * uint64(d.Segment.Multiplier())
}

// String returns the dart as players read it: 20, D20, T20, Bull, Bullseye, M7 (a miss
// beside the 7), Miss (a miss, unknown where). The web interfaces translate it.
func (d Dart) String() string {
	n := strconv.Itoa(int(d.Number))
	switch d.Segment {
	case Miss:
		if d.Number > 0 {
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
