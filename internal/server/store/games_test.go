package store

import (
	"context"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// Of the games on a board only those that may still be running are listed: started, not
// aborted, not completed. Games on other boards aren't.
func TestListUnfinishedGamesByBoard(t *testing.T) {
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	b, other := board.Board{AccountID: 1, Name: "B"}, board.Board{AccountID: 1, Name: "Other"}
	for _, x := range []*board.Board{&b, &other} {
		if err := st.CreateBoard(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	create := func(boardID int64, change func(g *game.Game) error) int64 {
		g := game.Game{
			Settings:     game.Settings{Mode: game.ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}},
			Participants: []game.Participant{{PlayerID: 1, BoardID: &boardID}},
		}
		if err := st.CreateGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		if change != nil {
			if err := change(&g); err != nil {
				t.Fatal(err)
			}
		}
		return g.ID
	}
	start := func(g *game.Game) error { g.StartedAt = &now; return st.StartGame(ctx, g) }

	running := create(b.ID, start)
	create(b.ID, nil) // in the lobby
	create(b.ID, func(g *game.Game) error {
		if err := start(g); err != nil {
			return err
		}
		g.AbortedAt = &now
		return st.AbortGame(ctx, g)
	})
	create(b.ID, func(g *game.Game) error {
		if err := start(g); err != nil {
			return err
		}
		g.CompletedAt = &now
		return st.CompleteGame(ctx, g)
	})
	create(other.ID, start)

	games, err := st.ListUnfinishedGamesByBoard(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].ID != running {
		t.Errorf("listed %d games, want only the running one (%d): %+v", len(games), running, games)
	}
}
