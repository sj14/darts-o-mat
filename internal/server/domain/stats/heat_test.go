package stats

import (
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

func TestHeat(t *testing.T) {
	t.Parallel()
	h := NewHeat([]dartboard.Point{{X: 1, Y: 1}, {X: 2, Y: 3}, {X: -1, Y: 1}, {X: 500, Y: 0}})
	if h.Darts != 3 || h.Max != 2 || len(h.Cells) != 2 {
		t.Fatalf("heat = %+v; want 3 darts (the far one left out) in 2 cells, 2 in the fullest", h)
	}
	if c := h.Cells[1]; c.Count != 2 || c.X != 4 || c.Y != 4 {
		t.Errorf("cell = %+v; want the square from 0 to 8 mm with 2 darts", c)
	}
}
