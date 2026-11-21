package service

import (
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
)

// newBotService is newService with bots that throw at once and the same darts every run.
func newBotService(t *testing.T) *Service {
	t.Helper()
	s := newService(t)
	s.botPause = 0
	s.bots = rand.New(rand.NewPCG(1, 2))
	return s
}

// lobbyWithBots creates a game of the account's own player in the lobby and adds bots with
// the spreads.
func lobbyWithBots(t *testing.T, s *Service, acc account.Account, g game.Game, spreads ...int) int64 {
	t.Helper()
	ctx := context.Background()
	v, err := s.CreateGame(ctx, acc, g, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, spread := range spreads {
		if err := s.AddBot(ctx, acc, v.Game.ID, spread); err != nil {
			t.Fatal(err)
		}
	}
	return v.Game.ID
}

// botsOf returns the players of the game that are bots.
func botsOf(g game.Game) []int64 {
	var bots []int64
	for _, p := range g.Participants {
		if p.BotSpread > 0 {
			bots = append(bots, p.PlayerID)
		}
	}
	return bots
}

// waitFor waits until the game's state meets the condition, as the bots throw by their
// timers.
func waitFor(t *testing.T, s *Service, gameID int64, what string, cond func(v GameView) bool) GameView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v := gameView(t, s, gameID)
		if cond(v) {
			return v
		}
		if time.Now().After(deadline) {
			t.Fatalf("waiting for %s: last entries %+v", what, v.Darts)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func upIs(playerID int64) func(v GameView) bool {
	return func(v GameView) bool {
		c := v.Common()
		return c.CurrentPlayerID != nil && *c.CurrentPlayerID == playerID && !c.WaitingForTakeout
	}
}

// A bot is added to the lobby with its spread; a second one is another bot. Bots don't show
// among the account's players, and a free bot is taken again in the next game.
func TestAddBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna := register(t, s, "anna")

	id := lobbyWithBots(t, s, anna, x01Game(), 20, 20)
	v, err := s.Game(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	bots := botsOf(v.Game)
	if len(bots) != 2 || bots[0] == bots[1] {
		t.Fatalf("bots %v, want two of them", bots)
	}
	if names := []string{v.Players[1].Name, v.Players[2].Name}; !slices.Equal(names, []string{"Bot", "Bot 2"}) {
		t.Errorf("bots' names %v, want Bot and Bot 2", names)
	}
	players, err := s.Players(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	if len(players) != 1 {
		t.Errorf("anna's players %v, want only her own: bots aren't guests to choose", players)
	}
	if len(v.Choices.Players) != 0 {
		t.Errorf("lobby offers %v to add, want nothing", v.Choices.Players)
	}

	// Leaving the lobby frees the bots: the next game takes the first again.
	if _, err := s.LeaveGame(ctx, anna, id); err != nil {
		t.Fatal(err)
	}
	next := lobbyWithBots(t, s, anna, x01Game(), 30)
	if v, _ = s.Game(ctx, anna, next); !slices.Equal(botsOf(v.Game), bots[:1]) {
		t.Errorf("bots of the next game %v, want the first again %v", botsOf(v.Game), bots[:1])
	}
	if p, _ := v.Game.Participant(bots[0]); p.BotSpread != 30 {
		t.Errorf("spread %d, want the new one 30: it belongs to the seat", p.BotSpread)
	}
}

// Bots sit only in games the players set up themselves, on no board, with a spread, and
// only next to a player of their account.
func TestBotSeats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)

	id := lobbyWithBots(t, s, anna, x01Game(), 20)
	v, _ := s.Game(ctx, anna, id)
	bot := botsOf(v.Game)[0]
	for _, spread := range []int{0, 81} {
		if err := s.AddBot(ctx, anna, id, spread); !errors.Is(err, ErrBotSpread) {
			t.Errorf("spread %d: %v, want ErrBotSpread", spread, err)
		}
	}
	b, err := s.CreateBoard(ctx, anna, "Board")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSeatBoard(ctx, anna, GameLobby(id), bot, &b.ID); !errors.Is(err, ErrBotBoard) {
		t.Errorf("bot on a board: %v, want ErrBotBoard", err)
	}

	// Ben is invited, not in yet: he can't add a bot.
	if err := s.Invite(ctx, anna, GameLobby(id), "ben"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddBot(ctx, ben, id, 20); !errors.Is(err, ErrForbidden) {
		t.Errorf("bot of an invited account: %v, want ErrForbidden", err)
	}

	open := x01Game()
	open.Lobby, open.TurnSeconds = game.Lobby{Seats: 3}, 60
	ov, err := s.CreateGame(ctx, ben, open, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddBot(ctx, ben, ov.Game.ID, 20); !errors.Is(err, ErrNoBots) {
		t.Errorf("bot in an open game: %v, want ErrNoBots", err)
	}

	// A bot can't be seated as a player, neither in a game nor in a tournament.
	carla := register(t, s, "carla")
	gv, err := s.CreateGame(ctx, carla, x01Game(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddPlayer(ctx, carla, GameLobby(gv.Game.ID), bot); err == nil {
		t.Error("carla added anna's bot")
	}
	if _, err := s.LeaveGame(ctx, anna, id); err != nil { // anna's own player sits there
		t.Fatal(err)
	}
	tv, err := s.CreateTournament(ctx, anna, tournament.Tournament{Format: tournament.Knockout, Settings: x01Game().Settings}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddPlayer(ctx, anna, TournamentLobby(tv.Tournament.ID), bot); !errors.Is(err, ErrNoBots) {
		t.Errorf("bot in a tournament: %v, want ErrNoBots", err)
	}
}

// Taking the account's own player out of the lobby takes its bots too: no lobby is left to
// bots.
func TestRemoveLastPlayerTakesBots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna := register(t, s, "anna")
	id := lobbyWithBots(t, s, anna, x01Game(), 20)
	if err := s.RemovePlayer(ctx, anna, GameLobby(id), anna.PlayerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Game(ctx, anna, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("lobby after anna went: %v, want it gone", err)
	}
}

// The bot throws its turn by itself, its darts with a position; on a board it pulls its
// own darts out.
func TestBotThrows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna := register(t, s, "anna")
	b, err := s.CreateBoard(ctx, anna, "Board")
	if err != nil {
		t.Fatal(err)
	}
	v, err := s.CreateGame(ctx, anna, x01Game(), 0, &b.ID)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	if err := s.AddBot(ctx, anna, id, 20); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(ctx, anna, GameLobby(id)); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Takeout(ctx, anna, id); err != nil {
		t.Fatal(err)
	}

	v = waitFor(t, s, id, "anna's next turn", upIs(anna.PlayerID))
	bot := botsOf(v.Game)[0]
	entries := v.Darts[4:] // after anna's turn and her takeout
	if len(entries) != 4 || !entries[3].Takeout {
		t.Fatalf("bot's turn %+v, want three darts and the takeout", entries)
	}
	for _, d := range entries[:3] {
		if d.PlayerID != bot || d.X == nil || d.Y == nil {
			t.Errorf("bot's dart %+v, want one of the bot with a position", d)
		}
	}
}

// When the last player who isn't a bot is skipped out of the game, the bots don't play on
// among themselves: the game is aborted.
func TestOnlyBotsLeft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna := register(t, s, "anna")
	var offset atomic.Int64 // the bots' timers read the time meanwhile
	s.now = func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }
	g := x01Game()
	g.TurnSeconds = 60
	id := lobbyWithBots(t, s, anna, g, 20, 20)
	if err := s.Start(ctx, anna, GameLobby(id)); err != nil {
		t.Fatal(err)
	}
	for range game.MaxSkips {
		waitFor(t, s, id, "anna's turn", upIs(anna.PlayerID))
		offset.Add(int64(2 * time.Minute))
		s.turnTimer(id)
	}
	if v := gameView(t, s, id); !v.Game.Aborted() {
		t.Errorf("game with only bots left: aborted %v, last entries %+v; want it aborted", v.Game.Aborted(), v.Darts[len(v.Darts)-1])
	}
}

// A rematch seats the bots again, at their strength.
func TestRematchWithBot(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newBotService(t)
	anna := register(t, s, "anna")
	g := game.Game{Settings: game.Settings{Mode: game.ModeX01, Rules: &x01.Settings{StartScore: 40, OutMode: x01.OutModeDouble}}}
	id := lobbyWithBots(t, s, anna, g, 25)
	if err := s.Start(ctx, anna, GameLobby(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDart(ctx, anna, id, throw.Dart{Number: 20, Segment: dartboard.Double}); err != nil {
		t.Fatal(err)
	}
	r, err := s.Rematch(ctx, anna, id)
	if err != nil {
		t.Fatal(err)
	}
	bots := botsOf(r.Game)
	if len(bots) != 1 {
		t.Fatalf("rematch's bots %v, want one", bots)
	}
	if p, _ := r.Game.Participant(bots[0]); p.BotSpread != 25 {
		t.Errorf("bot's spread in the rematch %d, want 25", p.BotSpread)
	}

	// The rematch completed the game anna won: it counts among the games against bots.
	for _, tc := range []struct {
		opponents   stats.Opponents
		games, wins int
	}{
		{stats.AgainstPeople, 0, 0},
		{stats.AgainstBots, 1, 1},
	} {
		sums, err := s.Stats(ctx, anna, "", tc.opponents)
		if err != nil {
			t.Fatal(err)
		}
		if x := sums[0].X01; x.Games != tc.games || x.Wins != tc.wins {
			t.Errorf("against %q: games %d, wins %d; want %d, %d", tc.opponents, x.Games, x.Wins, tc.games, tc.wins)
		}
	}
}
