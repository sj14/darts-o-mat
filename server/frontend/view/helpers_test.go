package view

import (
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/common/i18n"
	"github.com/sj14/darts-o-mat/server/domain/board"
	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/server/service"
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
	board1, board2 := int64(1), int64(2)
	const anna, ben, guest = int64(10), int64(20), int64(30)
	progress := func(current int64) progress {
		return progress{State: turn.State{CurrentPlayerID: &current}}
	}
	players := func(boards ...*int64) service.GameView {
		ids := []int64{anna, guest, ben}
		v := service.GameView{}
		for i, b := range boards {
			v.Game.Participants = append(v.Game.Participants, game.Participant{PlayerID: ids[i], Status: game.InvitationAccepted, BoardID: b})
		}
		return v
	}
	with := func(v service.GameView, mine ...int64) service.GameView {
		v.Mine = mine
		return v
	}

	tests := []struct {
		name      string
		v         service.GameView
		current   int64
		hand, add bool
	}{
		{"no board, my turn", with(players(nil, nil, nil), anna, guest), guest, true, false},
		{"no board, a friend's turn", with(players(nil, nil, nil), anna, guest), ben, false, false},
		{"at a board, my turn", with(players(&board1, &board1, nil), anna, guest), guest, false, true},
		{"on my own board", with(players(nil, nil, &board2), ben), ben, false, true},
		{"joined without a board next to a host with one", with(players(&board1, &board1, nil), ben), ben, true, false},
		{"watching a friend who has a board", with(players(nil, nil, &board2), anna, guest), ben, false, false},
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

func TestLastTurnOf(t *testing.T) {
	st := x01.State{Turns: []x01.Turn{
		{PlayerID: 1, Round: 1, Points: 125, Complete: true},
		{PlayerID: 2, Round: 1, Points: 0, Bust: true, Complete: true},
		{PlayerID: 1, Round: 2, Points: 60, Complete: true},
		{PlayerID: 2, Round: 2, Points: 20}, // still throwing
	}}
	v := service.GameView{State: st}
	texts := textsOf(i18n.English)

	if got, ok := lastTurnOf(texts, v, 1); !ok || got.Value != "60" || got.Strong || got.Bust {
		t.Errorf("player 1 = %+v, %v; want the 60", got, ok)
	}
	if got, ok := lastTurnOf(texts, v, 2); !ok || !got.Bust || got.Value != texts.Busted() {
		t.Errorf("player 2 = %+v, %v; want the bust (the unfinished turn doesn't count)", got, ok)
	}
	if _, ok := lastTurnOf(texts, v, 3); ok {
		t.Error("a player who didn't throw has a last turn")
	}
}
