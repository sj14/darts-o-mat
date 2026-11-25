package store

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
)

// Bots don't show among the account's players and guests, only in ListBots; a game with a
// bot has a timer like one with a time per turn.
func TestBots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st, err := NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count

	accountID := int64(1)
	own := player.Player{Name: "Anna"}
	guest := player.Player{Name: "Ben", GuestOf: &accountID}
	bot := player.Player{Name: "Bot", GuestOf: &accountID, Bot: true}
	otherBot := player.Player{Name: "Bot", GuestOf: new(int64(2)), Bot: true}
	for _, p := range []*player.Player{&own, &guest, &bot, &otherBot} {
		if err := st.StorePlayer(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	players, err := st.ListPlayers(ctx, accountID, own.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(players); !slices.Equal(got, []int64{own.ID, guest.ID}) {
		t.Errorf("players %v, want the own player and the guest %v", got, []int64{own.ID, guest.ID})
	}
	bots, err := st.ListBots(ctx, accountID)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(bots); !slices.Equal(got, []int64{bot.ID}) {
		t.Errorf("bots %v, want the account's bot %v", got, []int64{bot.ID})
	}

	now := time.Now()
	create := func(participants []game.Participant) int64 {
		g := game.Game{
			Settings:     game.Settings{Mode: game.ModeX01, Legs: 1, Sets: 1, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}},
			Participants: participants,
		}
		if err := st.CreateGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		g.StartedAt = &now
		if err := st.StartGame(ctx, &g); err != nil {
			t.Fatal(err)
		}
		return g.ID
	}
	withBot := create([]game.Participant{{PlayerID: own.ID}, {PlayerID: bot.ID, BotSpread: 200}})
	create([]game.Participant{{PlayerID: own.ID}, {PlayerID: guest.ID}})

	games, err := st.ListTimedGames(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 || games[0].ID != withBot {
		t.Fatalf("timed games %v, want only the game with the bot %d", games, withBot)
	}
	if p, _ := games[0].Participant(bot.ID); p.BotSpread != 200 {
		t.Errorf("bot's spread %d, want 200", p.BotSpread)
	}
}

func ids(players []player.Player) []int64 {
	var result []int64
	for _, p := range players {
		result = append(result, p.ID)
	}
	return result
}
