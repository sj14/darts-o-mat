package throw

import (
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/uptrace/bun"
)

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
	Number  uint8             `json:"number"`
	Segment dartboard.Segment `bun:",notnull" json:"segment,omitempty"`
	Takeout bool              `bun:",notnull,default:false" json:"takeout,omitempty"`
	// PlayerID is the player who threw the dart, set by the server from whose turn it was
	// (0 for takeouts); for a Skip or Leave entry the player whose turn was skipped, or who left. Replaying a game checks
	// that it is the player who is up.
	PlayerID int64 `bun:",notnull,default:0" json:"player_id,omitempty"`
	// Leave marks the entry of the log in which the player PlayerID left the game; the
	// others play on without them (see turn.Turn.Leave).
	Leave bool `bun:",notnull,default:false" json:"leave,omitempty"`
	// Skip marks the entry of the log in which the turn of the player PlayerID was skipped
	// because they took too long (game.Settings.TurnSeconds, see turn.Turn.Skip).
	Skip bool `bun:",notnull,default:false" json:"skip,omitempty"`
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

// IsDart reports whether the entry of a game's log is a dart, and not a takeout, a
// skipped turn or a player leaving.
func (d Dart) IsDart() bool {
	return !d.Takeout && !d.Leave && !d.Skip
}

// Hit returns the field and position of the dart, without the rest of the log entry.
func (d Dart) Hit() dartboard.Hit {
	return dartboard.Hit{Number: d.Number, Segment: d.Segment, X: d.X, Y: d.Y}
}

// FromHit returns the log entry for a dart the cameras saw, still without game and
// player.
func FromHit(h dartboard.Hit) Dart {
	return Dart{Number: h.Number, Segment: h.Segment, X: h.X, Y: h.Y}
}

// Validate checks that the dart hit a field that exists on the board.
func (d Dart) Validate() error { return d.Hit().Validate() }

// Score returns the points of the dart, 0 for a miss.
func (d Dart) Score() uint64 { return d.Hit().Score() }

// String returns the dart as players read it, see dartboard.Hit.String.
func (d Dart) String() string { return d.Hit().String() }
