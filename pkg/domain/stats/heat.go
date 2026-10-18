package stats

import (
	"math"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
)

// HeatCellSize is the side of a square of the heatmap in mm: small enough to tell the
// triple ring (8 mm wide) from its neighbours, large enough to keep the number of
// squares small.
const HeatCellSize = 8.0

// HeatReach is how far from the bull a position still counts (mm): the board is 170 mm
// wide, a dart further out is a miss that doesn't tell anything about the aim.
const HeatReach = 200.0

// HeatCell is a square of the heatmap with the darts that hit it. X and Y are its
// middle in mm on the board (origin in the bull, Y towards the 20).
type HeatCell struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Count int     `json:"count"`
}

// Heat shows where a player's darts hit the board, from the positions the cameras saw.
type Heat struct {
	Darts int        `json:"darts"` // darts in the cells
	Max   int        `json:"max"`   // darts in the fullest cell
	Cells []HeatCell `json:"cells"`
}

// NewHeat counts the positions per square.
func NewHeat(points []dartboard.Point) Heat {
	type key struct{ x, y int }
	counts := map[key]int{}
	var h Heat
	for _, p := range points {
		if p.Radius() > HeatReach {
			continue
		}
		counts[key{int(math.Floor(p.X / HeatCellSize)), int(math.Floor(p.Y / HeatCellSize))}]++
		h.Darts++
	}
	for k, n := range counts {
		h.Cells = append(h.Cells, HeatCell{
			X:     (float64(k.x) + 0.5) * HeatCellSize,
			Y:     (float64(k.y) + 0.5) * HeatCellSize,
			Count: n,
		})
		h.Max = max(h.Max, n)
	}
	slices.SortFunc(h.Cells, func(a, b HeatCell) int {
		if a.Y != b.Y {
			return int(b.Y - a.Y)
		}
		return int(a.X - b.X)
	})
	return h
}
