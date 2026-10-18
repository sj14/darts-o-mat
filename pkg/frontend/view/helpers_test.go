package view

import (
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/board"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/turn"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func TestDefaultBoard(t *testing.T) {
	view := func(id int64, used time.Duration) board.Board {
		b := board.Board{ID: id}
		if used != 0 {
			at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(used)
			b.LastUsedAt = &at
		}
		return b
	}
	tests := []struct {
		name   string
		boards []board.Board
		want   int64
	}{
		{"no boards", nil, 0},
		{"never used: the first", []board.Board{view(3, 0), view(5, 0)}, 3},
		{"the last used", []board.Board{view(3, time.Hour), view(5, 2*time.Hour), view(7, 0)}, 5},
		{"a used one over unused ones before it", []board.Board{view(3, 0), view(5, time.Hour)}, 5},
	}
	for _, tt := range tests {
		if got := defaultBoard(tt.boards); got != tt.want {
			t.Errorf("%s: defaultBoard = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestHandEntry(t *testing.T) {
	hostBoard, benBoard := int64(1), int64(2)
	const anna, ben, guest = int64(10), int64(20), int64(30)
	progress := func(current int64) progress {
		return progress{State: turn.State{CurrentPlayerID: &current}}
	}
	invitation := func(player int64, board *int64) game.Invitation {
		return game.Invitation{PlayerID: player, Status: game.InvitationAccepted, BoardID: board}
	}

	tests := []struct {
		name      string
		v         service.GameView
		current   int64
		hand, add bool
	}{
		{"no board, my turn", service.GameView{IsOwner: true, Mine: []int64{anna, guest}}, guest, true, false},
		{"no board, a friend's turn", service.GameView{IsOwner: true, Mine: []int64{anna, guest}}, ben, false, false},
		{"host's board, my turn", service.GameView{IsOwner: true, Game: game.Game{Settings: game.Settings{BoardID: &hostBoard}}, Mine: []int64{anna, guest}}, guest, false, true},
		{"on my own board", service.GameView{Invitations: []game.Invitation{invitation(ben, &benBoard)}, Mine: []int64{ben}, CanThrow: true}, ben, false, true},
		{"joined without a board next to a host with one", service.GameView{Game: game.Game{Settings: game.Settings{BoardID: &hostBoard}}, Invitations: []game.Invitation{invitation(ben, nil)}, Mine: []int64{ben}}, ben, true, false},
		{"watching a friend who has a board", service.GameView{Invitations: []game.Invitation{invitation(ben, &benBoard)}, Mine: []int64{anna}, IsOwner: true}, ben, false, false},
	}
	for _, tt := range tests {
		tt.v.CanThrow = true
		p := progress(tt.current)
		if got := handEntry(tt.v, p); got != tt.hand {
			t.Errorf("%s: handEntry = %v, want %v", tt.name, got, tt.hand)
		}
		if got := mayAddDart(tt.v, p); got != tt.add {
			t.Errorf("%s: mayAddDart = %v, want %v", tt.name, got, tt.add)
		}
	}
}
