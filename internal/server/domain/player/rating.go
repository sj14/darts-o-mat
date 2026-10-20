package player

import "github.com/uptrace/bun"

// MinRatings is how many thumbs, from different players, a player needs before others see the share of the good
// ones: with fewer one angry loser would be the whole picture.
const MinRatings = 3

// Rating is the thumb one player gave another after a game (table ratings): up for a good
// opponent, down for a bad one. There is one per game and pair, changing it replaces it;
// in a reputation only the newest per pair counts.
// Only players with an account rate and are rated, not guests.
type Rating struct {
	bun.BaseModel `bun:"table:ratings"`

	GameID  int64 `bun:",pk" json:"-"`
	RaterID int64 `bun:",pk" json:"-"`
	RatedID int64 `bun:",pk" json:"player_id"`
	Up      bool  `bun:",notnull" json:"up"`
}

// Reputation is the thumbs a player got from others.
type Reputation struct {
	Up   int `json:"up"`
	Down int `json:"down"`
}

// Total is the number of thumbs.
func (r Reputation) Total() int {
	return r.Up + r.Down
}

// Share is the share of thumbs up, 0 without any thumbs.
func (r Reputation) Share() float64 {
	if r.Total() == 0 {
		return 0
	}
	return float64(r.Up) / float64(r.Total())
}

// Shown reports whether there are enough thumbs (MinRatings) to show the share to others.
func (r Reputation) Shown() bool {
	return r.Total() >= MinRatings
}
