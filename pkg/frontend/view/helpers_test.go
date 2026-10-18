package view

import (
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func TestDefaultBoard(t *testing.T) {
	view := func(id int64, used time.Duration) service.BoardView {
		b := board.Board{ID: id}
		if used != 0 {
			at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(used)
			b.LastUsedAt = &at
		}
		return service.BoardView{Board: b}
	}
	tests := []struct {
		name   string
		boards []service.BoardView
		want   int64
	}{
		{"no boards", nil, 0},
		{"never used: the first", []service.BoardView{view(3, 0), view(5, 0)}, 3},
		{"the last used", []service.BoardView{view(3, time.Hour), view(5, 2*time.Hour), view(7, 0)}, 5},
		{"a used one over unused ones before it", []service.BoardView{view(3, 0), view(5, time.Hour)}, 5},
	}
	for _, tt := range tests {
		if got := defaultBoard(tt.boards); got != tt.want {
			t.Errorf("%s: defaultBoard = %d, want %d", tt.name, got, tt.want)
		}
	}
}
