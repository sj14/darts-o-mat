package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/warning"
	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/turn"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/sj14/darts-o-mat/internal/server/store"
	"golang.org/x/crypto/bcrypt"
)

func newService(t *testing.T) *Service {
	t.Helper()
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })                    // the in-memory database is gone with it, also for -count
	s := New(st, Options{PasswordCost: bcrypt.MinCost}) // fast hashes
	s.shuffle = func(int, func(i, j int)) {}            // the order the players took their seats
	return s
}

func register(t *testing.T, s *Service, username string) account.Account {
	t.Helper()
	a, err := s.Register(context.Background(), username, "", "", "password")
	if err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	return a
}

// befriend makes a and b friends.
func befriend(t *testing.T, s *Service, a, b account.Account) {
	t.Helper()
	ctx := context.Background()
	if err := s.RequestFriend(ctx, a, b.Username); err != nil {
		t.Fatalf("%s requests %s: %v", a.Username, b.Username, err)
	}
	if err := s.AcceptFriend(ctx, b, a.ID); err != nil {
		t.Fatalf("%s accepts %s: %v", b.Username, a.Username, err)
	}
}

func x01Game(playerIDs ...int64) game.Game {
	g := game.Game{Settings: game.Settings{Mode: game.ModeX01, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}}}
	for _, id := range playerIDs {
		g.Participants = append(g.Participants, game.Participant{PlayerID: id})
	}
	return g
}

// withBoard puts all players of the game on the board.
func withBoard(g game.Game, boardID int64) game.Game {
	for i := range g.Participants {
		g.Participants[i].BoardID = &boardID
	}
	return g
}

var t20 = throw.Dart{Number: 20, Segment: dartboard.Triple}

func TestAuth(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)

	a := register(t, s, " Anna ")
	if a.Username != "anna" {
		t.Errorf("username = %q, want it normalized to anna", a.Username)
	}
	if _, err := s.Register(ctx, "ANNA", "", "", "password"); !errors.Is(err, account.ErrUsernameTaken) {
		t.Errorf("duplicate username: got %v, want account.ErrUsernameTaken", err)
	}
	if _, err := s.Register(ctx, "ben", "", "", "short"); !errors.Is(err, ErrInvalid) {
		t.Errorf("short password: got %v, want ErrInvalid", err)
	}

	if _, _, err := s.Login(ctx, "anna", "wrong-password"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("wrong password: got %v, want ErrUnauthenticated", err)
	}
	if _, _, err := s.Login(ctx, "nobody", "password"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("unknown user: got %v, want ErrUnauthenticated", err)
	}

	token, _, err := s.Login(ctx, "Anna", "password")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Authenticate(ctx, token)
	if err != nil || got.ID != a.ID {
		t.Fatalf("authenticate: got %v, %v", got, err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("after logout: got %v, want ErrUnauthenticated", err)
	}

	// Only the admin (the first account) closes the registration.
	ben, err := s.Register(ctx, "ben", "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRegistration(ctx, ben, false); !errors.Is(err, ErrForbidden) {
		t.Errorf("close registration as ben: got %v, want ErrForbidden", err)
	}
	if err := s.SetRegistration(ctx, a, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, "cara", "", "", "password"); !errors.Is(err, ErrRegistrationClosed) {
		t.Errorf("closed registration: got %v, want ErrRegistrationClosed", err)
	}
	// The setting is stored, not only kept in memory.
	if open, err := New(s.store, Options{PasswordCost: bcrypt.MinCost}).RegistrationOpen(ctx); err != nil || open {
		t.Errorf("registration after restart: open %v, %v; want closed", open, err)
	}
	if err := s.SetRegistration(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register(ctx, "cara", "", "", "password"); err != nil {
		t.Errorf("reopened registration: %v", err)
	}
}

func TestEmail(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)

	// The email is optional when registering, and checked if given.
	if a, err := s.Register(ctx, "anna", "", "", "password"); err != nil || a.Email != "" {
		t.Fatalf("register without an email: %q, %v", a.Email, err)
	}
	for _, bad := range []string{"ben", "ben@", "@example.com", "ben@example", "Ben <ben@example.com>", "ben @example.com"} {
		if _, err := s.Register(ctx, "ben", bad, "", "password"); !errors.Is(err, account.ErrInvalidEmail) || !errors.Is(err, ErrInvalid) {
			t.Errorf("register with %q: got %v, want ErrInvalid wrapping ErrInvalidEmail", bad, err)
		}
	}
	ben, err := s.Register(ctx, "ben", " Ben@Example.com ", "", "password")
	if err != nil || ben.Email != "ben@example.com" {
		t.Fatalf("register with an email: %q, %v; want it normalized", ben.Email, err)
	}

	// An email belongs to one account; many accounts can have none.
	if _, err := s.Register(ctx, "cara", "BEN@example.com", "", "password"); !errors.Is(err, account.ErrEmailTaken) {
		t.Errorf("register with an email in use: got %v, want account.ErrEmailTaken", err)
	}
	if _, err := s.Register(ctx, "dora", "", "", "password"); err != nil {
		t.Errorf("a second account without an email: %v", err)
	}
	cara, _ := s.Register(ctx, "cara", "cara@example.com", "", "password")
	if _, err := s.SetEmail(ctx, cara, "ben@example.com"); !errors.Is(err, account.ErrEmailTaken) {
		t.Errorf("change to an email in use: got %v, want account.ErrEmailTaken", err)
	}
	if a, err := s.SetEmail(ctx, ben, "ben@example.com"); err != nil || a.Email != "ben@example.com" {
		t.Errorf("save your own email again: %q, %v", a.Email, err)
	}

	// It can be changed and removed in the settings; the account is stored with it.
	if _, err := s.SetEmail(ctx, ben, "nope"); !errors.Is(err, account.ErrInvalidEmail) {
		t.Errorf("change to an invalid email: got %v", err)
	}
	if a, err := s.SetEmail(ctx, ben, "Ben.New@example.org"); err != nil || a.Email != "ben.new@example.org" {
		t.Fatalf("change: %q, %v", a.Email, err)
	}
	if _, _, err := s.Login(ctx, "ben", "password"); err != nil {
		t.Errorf("login after changing the email: %v", err)
	}
	token, _, _ := s.Login(ctx, "ben", "password")
	if a, _ := s.Authenticate(ctx, token); a.Email != "ben.new@example.org" {
		t.Errorf("stored email = %q, want ben.new@example.org", a.Email)
	}
	if a, err := s.SetEmail(ctx, ben, ""); err != nil || a.Email != "" {
		t.Errorf("remove: %q, %v", a.Email, err)
	}
}

func TestGuestsBelongToTheirAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	guest, err := s.CreateGuest(ctx, anna, "Papa")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"papa", " Papa ", "Anna"} {
		if _, err := s.CreateGuest(ctx, anna, name); !errors.Is(err, ErrGuestNameTaken) {
			t.Errorf("CreateGuest(%q) = %v, want ErrGuestNameTaken", name, err)
		}
	}

	players, _ := s.Players(ctx, anna)
	if len(players) != 2 || players[0].ID != anna.PlayerID || players[1].ID != guest.ID {
		t.Errorf("anna's players = %+v, want herself followed by her guest", players)
	}

	if _, err := s.CreateGame(ctx, ben, x01Game(), guest.ID, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben begins with anna's guest: got %v, want ErrInvalid", err)
	}
	v, err := s.CreateGame(ctx, ben, x01Game(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	lobby := GameLobby(v.Game.ID)
	if err := s.AddPlayer(ctx, ben, lobby, guest.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben adds anna's guest: got %v, want ErrInvalid", err)
	}
	if err := s.AddPlayer(ctx, ben, lobby, anna.PlayerID); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben adds anna without invitation: got %v, want ErrInvalid", err)
	}
	if err := s.Invite(ctx, ben, lobby, "nobody"); !isError[UnknownUserError](err) || !errors.Is(err, ErrInvalid) {
		t.Errorf("invite unknown user: got %v, want UnknownUserError wrapping ErrInvalid", err)
	}
	if err := s.Invite(ctx, ben, lobby, "Ben"); !errors.Is(err, ErrInviteSelf) {
		t.Errorf("invite yourself: got %v, want ErrInviteSelf", err)
	}
}

func TestInvitationFlow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	befriend(t, s, anna, ben)

	v, err := createGame(ctx, s, anna, x01Game(anna.PlayerID, guest.ID), []string{"Ben"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	if want := []int64{anna.PlayerID, guest.ID, ben.PlayerID}; !slices.Equal(v.Game.PlayerIDs(), want) {
		t.Errorf("player order = %v, want %v", v.Game.PlayerIDs(), want)
	}
	if v.Started {
		t.Error("game is ready although ben hasn't accepted")
	}

	if _, err := s.AddDart(ctx, anna, id, t20); !errors.Is(err, ErrNotReady) {
		t.Errorf("dart before acceptance: got %v, want ErrNotReady", err)
	}
	if _, err := s.Game(ctx, cara, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("outsider views game: got %v, want ErrNotFound", err)
	}

	invitations, _ := s.Invitations(ctx, ben)
	if len(invitations) != 1 || !invitations[0].Invited || invitations[0].CanThrow {
		t.Fatalf("ben's invitations = %+v", invitations)
	}
	if _, err := s.AddDart(ctx, ben, id, t20); !errors.Is(err, ErrForbidden) {
		t.Errorf("dart by pending invitee: got %v, want ErrForbidden", err)
	}

	v, err = acceptGame(ctx, s, ben, id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Started || !v.CanThrow {
		t.Errorf("after accepting: ready=%v canThrow=%v, want both", v.Started, v.CanThrow)
	}
	if _, err := acceptGame(ctx, s, ben, id, nil); !errors.Is(err, ErrStarted) {
		t.Errorf("accepting twice: got %v, want ErrStarted", err)
	}

	if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
		t.Errorf("dart by owner: %v", err)
	}
	if _, err := s.AddDart(ctx, ben, id, t20); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben enters a dart of anna's turn: got %v, want ErrForbidden", err)
	}
	for range 5 { // the rest of anna's visit and the one of her guest
		if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.AddDart(ctx, ben, id, t20); err != nil {
		t.Errorf("dart by accepted invitee on his turn: %v", err)
	}
	if _, err := s.UndoDart(ctx, anna, id); !errors.Is(err, ErrForbidden) {
		t.Errorf("anna takes back ben's dart: got %v, want ErrForbidden", err)
	}
	if _, err := s.UndoDart(ctx, ben, id); err != nil {
		t.Errorf("ben takes back his dart: %v", err)
	}
	if _, err := s.UndoDart(ctx, cara, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("undo by outsider: got %v, want ErrNotFound", err)
	}

	games, _ := s.ListGames(ctx, ben, 10)
	if len(games) != 1 || games[0].Game.ID != id {
		t.Errorf("ben's games = %+v, want the game he joined", games)
	}
	if games, _ := s.ListGames(ctx, cara, 10); len(games) != 0 {
		t.Errorf("cara's games = %+v, want none", games)
	}
}

func TestDeclineAndWithdraw(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, anna, ben)
	befriend(t, s, cara, anna)

	v, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), []string{"ben", "cara"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID

	if _, err := withdrawGame(ctx, s, ben, id, cara.PlayerID); !errors.Is(err, ErrForbidden) {
		t.Errorf("withdraw by a player who didn't answer yet: got %v, want ErrForbidden", err)
	}

	if err := s.Decline(ctx, ben, GameLobby(id)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Game(ctx, ben, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben after declining: got %v, want ErrNotFound", err)
	}

	v, err = withdrawGame(ctx, s, anna, id, cara.PlayerID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(v.Game.PlayerIDs(), []int64{anna.PlayerID}) || !v.Started {
		t.Errorf("after withdrawing: players=%v ready=%v, want only anna and ready", v.Game.PlayerIDs(), v.Started)
	}
}

func TestGameWithoutPlayersLeftIsDeleted(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)

	// Everybody who creates a game takes part in it.
	if _, err := createGame(ctx, s, anna, x01Game(), []string{"ben"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a game without a player of the creator: got %v, want ErrInvalid", err)
	}

	// A game that nobody accepted is gone when the last player leaves it.
	v, err := createGame(ctx, s, anna, x01Game(anna.PlayerID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.LeaveGame(ctx, anna, v.Game.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Decline(ctx, ben, GameLobby(v.Game.ID)); !errors.Is(err, ErrNotFound) {
		t.Errorf("declining a game that is gone: got %v, want ErrNotFound", err)
	}
}

func TestBoards(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	b, err := s.CreateBoard(ctx, anna, "Keller")
	if err != nil {
		t.Fatal(err)
	}

	g := x01Game(ben.PlayerID)
	g = withBoard(g, b.ID)
	if _, err := createGame(ctx, s, ben, g, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben uses anna's board: got %v, want ErrInvalid", err)
	}
	if err := s.DeleteBoard(ctx, ben, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben deletes anna's board: got %v, want ErrNotFound", err)
	}

	if boards, _ := s.Boards(ctx, anna); boards[0].LastUsedAt != nil {
		t.Errorf("unused board has LastUsedAt %v", boards[0].LastUsedAt)
	}
	g = x01Game(anna.PlayerID)
	g = withBoard(g, b.ID)
	created, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatalf("anna uses her board: %v", err)
	}
	if boards, _ := s.Boards(ctx, anna); boards[0].LastUsedAt == nil || !boards[0].LastUsedAt.Equal(created.Game.CreatedAt) {
		t.Errorf("LastUsedAt = %v, want the game's creation %v", boards[0].LastUsedAt, created.Game.CreatedAt)
	}
}

func TestFriends(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")

	v, err := s.CreateGame(ctx, anna, x01Game(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	lobby := GameLobby(v.Game.ID)
	if err := s.Invite(ctx, anna, lobby, "ben"); !isError[NotFriendError](err) {
		t.Errorf("invite before befriending: got %v, want NotFriendError", err)
	}

	if err := s.RequestFriend(ctx, anna, "Ben"); err != nil {
		t.Fatal(err)
	}
	if err := s.RequestFriend(ctx, anna, "ben"); !errors.Is(err, ErrAlreadyFriends) {
		t.Errorf("second request: got %v, want ErrAlreadyFriends", err)
	}
	if err := s.RequestFriend(ctx, anna, "anna"); !errors.Is(err, ErrInvalid) {
		t.Errorf("befriend yourself: got %v, want ErrInvalid", err)
	}
	if err := s.RequestFriend(ctx, anna, "nobody"); !isError[UnknownUserError](err) {
		t.Errorf("unknown user: got %v, want UnknownUserError", err)
	}

	annas, _ := s.Friends(ctx, anna)
	bens, _ := s.Friends(ctx, ben)
	if len(annas.Outgoing) != 1 || annas.Outgoing[0].Username != "ben" || len(bens.Incoming) != 1 || bens.Incoming[0].Username != "anna" {
		t.Fatalf("pending request: anna %+v, ben %+v", annas, bens)
	}
	if err := s.Invite(ctx, anna, lobby, "ben"); !isError[NotFriendError](err) {
		t.Errorf("invite with pending request: got %v, want NotFriendError", err)
	}
	if err := s.AcceptFriend(ctx, anna, ben.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("requester accepts own request: got %v, want ErrNotFound", err)
	}

	// Ben requesting back accepts Anna's request.
	if err := s.RequestFriend(ctx, ben, "anna"); err != nil {
		t.Fatal(err)
	}
	annas, _ = s.Friends(ctx, anna)
	if len(annas.Friends) != 1 || annas.Friends[0].Name != "ben" || len(annas.Outgoing) != 0 {
		t.Fatalf("after mutual request: %+v", annas)
	}
	if err := s.Invite(ctx, anna, lobby, "ben"); err != nil {
		t.Errorf("invite friend: %v", err)
	}
	if err := s.Invite(ctx, anna, lobby, "cara"); !isError[NotFriendError](err) {
		t.Errorf("invite stranger: got %v, want NotFriendError", err)
	}

	if err := s.RemoveFriend(ctx, ben, anna.ID); err != nil {
		t.Fatal(err)
	}
	if annas, _ = s.Friends(ctx, anna); len(annas.Friends) != 0 {
		t.Errorf("after removing: %+v", annas)
	}
	// The game they already share stays.
	if games, _ := s.ListGames(ctx, ben, 10); len(games) != 1 {
		t.Errorf("ben's games after unfriending = %d, want 1", len(games))
	}

	// Declining a request removes it.
	_ = s.RequestFriend(ctx, cara, "anna")
	if err := s.RemoveFriend(ctx, anna, cara.ID); err != nil {
		t.Fatal(err)
	}
	if caras, _ := s.Friends(ctx, cara); len(caras.Outgoing) != 0 {
		t.Errorf("cara's request after anna declined: %+v", caras)
	}
}

func TestStats(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")

	g := x01Game(anna.PlayerID, guest.ID)
	g.Rules.(*x01.Settings).StartScore = 40
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Anna checks out with her first dart and wins at once.
	if _, err := s.AddDart(ctx, anna, v.Game.ID, throw.Dart{Number: 20, Segment: dartboard.Double}); err != nil {
		t.Fatal(err)
	}

	// A running game doesn't count.
	running, _ := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil)
	if _, err := s.AddDart(ctx, anna, running.Game.ID, t20); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.Stats(ctx, anna, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want anna and her guest", len(summaries))
	}
	if got := summaries[0].X01; got.Games != 1 || got.Wins != 1 || got.Darts != 1 || got.Average != 120 || got.HighestCheckout != 40 || got.BestLeg != 1 {
		t.Errorf("anna's stats = %+v", got)
	}
	if got := summaries[1].X01; got.Games != 1 || got.Wins != 0 || got.Darts != 0 || got.Average != 0 || got.BestLeg != 0 {
		t.Errorf("guest's stats = %+v", got)
	}
}

func TestBoardCamera(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	verein, _ := s.CreateBoard(ctx, anna, "Verein")

	if _, err := s.CreateBoardToken(ctx, ben, keller.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("token for someone else's board: got %v, want ErrNotFound", err)
	}
	token, err := s.CreateBoardToken(ctx, anna, keller.ID)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AuthenticateBoard(ctx, token)
	if err != nil || b.ID != keller.ID {
		t.Fatalf("authenticate board: got %+v, %v", b, err)
	}
	if _, err := s.AuthenticateBoard(ctx, "wrong"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("wrong token: got %v, want ErrUnauthenticated", err)
	}
	// A new token replaces the old one.
	if _, err := s.CreateBoardToken(ctx, anna, keller.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthenticateBoard(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("replaced token: got %v, want ErrUnauthenticated", err)
	}

	if _, err := s.AddBoardDart(ctx, keller, t20); !errors.Is(err, ErrNoActiveGame) {
		t.Errorf("no game: got %v, want ErrNoActiveGame", err)
	}

	// Each game is of another player of anna's: a player plays one game at a time.
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	oma, _ := s.CreateGuest(ctx, anna, "Oma")
	onBoard := func(playerID, boardID int64, invite ...string) GameView {
		t.Helper()
		g := withBoard(x01Game(playerID), boardID)
		v, err := createGame(ctx, s, anna, g, invite)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	game := onBoard(anna.PlayerID, keller.ID)
	other := onBoard(papa.ID, verein.ID)
	befriend(t, s, anna, ben)
	v, err := s.AddBoardDart(ctx, keller, t20)
	if err != nil {
		t.Fatal(err)
	}
	if v.Game.ID != game.Game.ID || len(v.Darts) != 1 {
		t.Errorf("dart went to game %d with %d darts, want game %d with 1", v.Game.ID, len(v.Darts), game.Game.ID)
	}
	if g, _ := s.Game(ctx, anna, other.Game.ID); len(g.Darts) != 0 {
		t.Error("a game on another board got a dart")
	}

	// A new game takes the board at once, even while it waits for ben: the cameras'
	// darts go nowhere until ben accepts.
	waiting := onBoard(oma.ID, keller.ID, "ben")
	if _, err := s.AddBoardDart(ctx, keller, t20); !errors.Is(err, ErrNoActiveGame) {
		t.Errorf("dart while the new game waits: got %v, want ErrNoActiveGame", err)
	}
	if g, _ := s.Game(ctx, anna, game.Game.ID); g.HasBoard() {
		t.Error("the first game kept the board")
	}
	if _, err := acceptGame(ctx, s, ben, waiting.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	if v, err = s.AddBoardDart(ctx, keller, t20); err != nil || v.Game.ID != waiting.Game.ID || len(v.Darts) != 1 {
		t.Fatalf("dart after ben accepted: game %d with %d darts, %v; want game %d with 1", v.Game.ID, len(v.Darts), err, waiting.Game.ID)
	}

	// One dart stuck, two bounced off: pulling out completes the turn with misses.
	v, err = s.BoardTakeout(ctx, keller)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Darts) != 4 || v.Darts[1].Segment != dartboard.Miss || v.Darts[2].Segment != dartboard.Miss || !v.Darts[3].Takeout {
		t.Errorf("after takeout: %d entries %+v, want the dart, 2 misses and the takeout", len(v.Darts), v.Darts)
	}
	// Nothing to pull out anymore: the log stays as it is.
	if v, _ = s.BoardTakeout(ctx, keller); len(v.Darts) != 4 {
		t.Errorf("second takeout changed the log: %d entries", len(v.Darts))
	}
	if v.BoardNames[oma.ID] != "Keller" {
		t.Errorf("board names = %v, want Keller", v.BoardNames)
	}
}

func TestBoardWarnings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	v, err := createGame(ctx, s, anna, withBoard(x01Game(anna.PlayerID), keller.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	changes, stop := s.Subscribe(v.Game.ID)
	defer stop()
	pending := func() bool {
		select {
		case <-changes:
			return true
		default:
			return false
		}
	}
	report := func(warnings ...warning.Warning) []board.Board {
		t.Helper()
		b, err := s.store.GetBoard(ctx, keller.ID) // as auth.RequireBoard loads it
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetBoardWarnings(ctx, b, warnings); err != nil {
			t.Fatal(err)
		}
		g, err := s.Game(ctx, anna, v.Game.ID)
		if err != nil {
			t.Fatal(err)
		}
		return g.CameraWarnings
	}

	if w := report(); len(w) != 0 || pending() {
		t.Errorf("no warnings: boards %+v; want none and no notification", w)
	}
	missing := warning.CamerasMissing{Cameras: 3, Watching: 2}
	grid := warning.GridMismatch{Camera: "cam2"}
	if w := report(missing, grid); len(w) != 1 || !slices.Equal(w[0].Warnings, warning.List{missing, grid}) || !pending() {
		t.Errorf("two warnings: boards %+v; want Keller with both and a notification", w)
	}
	if report(missing, grid); pending() {
		t.Error("notified although nothing changed")
	}
	if w := report(); len(w) != 0 || !pending() {
		t.Errorf("warnings gone: boards %+v; want none and a notification", w)
	}

	tooMany := make(warning.List, MaxWarnings+1)
	for i := range tooMany {
		tooMany[i] = missing
	}
	for name, warnings := range map[string]warning.List{
		"too many":    tooMany,
		"long camera": {warning.GridMismatch{Camera: strings.Repeat("x", maxWarningSize)}},
		"no code":     {warning.Unknown{}},
	} {
		if err := s.SetBoardWarnings(ctx, keller, warnings); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}

func TestSubscribe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	v, _ := createGame(ctx, s, anna, x01Game(anna.PlayerID), nil)
	other, _ := createGame(ctx, s, anna, x01Game(papa.ID), nil)

	changes, stop := s.Subscribe(v.Game.ID)
	pending := func() bool {
		select {
		case <-changes:
			return true
		default:
			return false
		}
	}

	if _, err := s.AddDart(ctx, anna, other.Game.ID, t20); err != nil {
		t.Fatal(err)
	}
	if pending() {
		t.Error("notified about another game")
	}

	// Two changes while the subscriber is busy arrive as one notification.
	_, _ = s.AddDart(ctx, anna, v.Game.ID, t20)
	_, _ = s.UndoDart(ctx, anna, v.Game.ID)
	if !pending() {
		t.Error("no notification after a dart")
	}
	if pending() {
		t.Error("changes were not merged into one notification")
	}

	stop()
	_, _ = s.AddDart(ctx, anna, v.Game.ID, t20)
	if pending() {
		t.Error("notified after stopping")
	}
}

func TestTakeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")

	g := x01Game(anna.PlayerID, guest.ID)
	g = withBoard(g, keller.ID)
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	current := func(v GameView) int64 { return *v.State.(x01.State).CurrentPlayerID }

	for range 3 {
		if v, err = s.AddDart(ctx, anna, id, t20); err != nil {
			t.Fatal(err)
		}
	}
	if !v.Common().WaitingForTakeout || current(v) != anna.PlayerID {
		t.Fatalf("after 3 darts: waiting=%v current=%d, want anna waiting", v.Common().WaitingForTakeout, current(v))
	}
	if _, err := s.AddDart(ctx, anna, id, t20); !errors.Is(err, turn.ErrWaitingForTakeout) {
		t.Errorf("fourth dart: got %v, want ErrWaitingForTakeout", err)
	}

	if v, err = s.Takeout(ctx, anna, id); err != nil {
		t.Fatal(err)
	}
	if v.Common().WaitingForTakeout || current(v) != guest.ID {
		t.Errorf("after takeout: waiting=%v current=%d, want the guest", v.Common().WaitingForTakeout, current(v))
	}
	// Another takeout without darts thrown changes nothing.
	if v, _ = s.Takeout(ctx, anna, id); len(v.Darts) != 4 {
		t.Errorf("empty takeout was recorded: %d log entries, want 3 darts and 1 takeout", len(v.Darts))
	}

	// Undo takes back the takeout first, then darts.
	if v, err = s.UndoDart(ctx, anna, id); err != nil {
		t.Fatal(err)
	}
	if !v.Common().WaitingForTakeout || len(v.Darts) != 3 {
		t.Errorf("after undo: waiting=%v entries=%d, want waiting with 3 darts", v.Common().WaitingForTakeout, len(v.Darts))
	}
	if v, _ = s.UndoDart(ctx, anna, id); len(v.Darts) != 2 || v.Common().WaitingForTakeout {
		t.Errorf("second undo: darts=%d waiting=%v, want 2 darts", len(v.Darts), v.Common().WaitingForTakeout)
	}

	// Without a board, the turn passes on right after the third dart (other players: these
	// two play already).
	oma, _ := s.CreateGuest(ctx, anna, "Oma")
	opa, _ := s.CreateGuest(ctx, anna, "Opa")
	free, _ := createGame(ctx, s, anna, x01Game(oma.ID, opa.ID), nil)
	for range 3 {
		v, _ = s.AddDart(ctx, anna, free.Game.ID, t20)
	}
	if v.Common().WaitingForTakeout || current(v) != opa.ID {
		t.Errorf("game without board: waiting=%v current=%d, want the guest", v.Common().WaitingForTakeout, current(v))
	}
}

// A camera dart while the game waits for the takeout means the cameras missed it: the
// takeout is entered, and the dart is the next player's.
func TestBoardDartAfterMissedTakeout(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	g := x01Game(anna.PlayerID, guest.ID)
	g = withBoard(g, keller.ID)
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if v, err = s.AddBoardDart(ctx, keller, t20); err != nil {
			t.Fatal(err)
		}
	}
	if !v.Common().WaitingForTakeout {
		t.Fatal("not waiting for the takeout after 3 darts")
	}
	if v, err = s.AddBoardDart(ctx, keller, t20); err != nil {
		t.Fatalf("fourth camera dart: %v", err)
	}
	st := v.State.(x01.State)
	if len(v.Darts) != 5 || !v.Darts[3].Takeout || *st.CurrentPlayerID != guest.ID || v.Common().DartsInTurn != 1 {
		t.Errorf("%d entries, current %d, %d darts in the turn; want 3 darts, the takeout and the guest's first dart", len(v.Darts), *st.CurrentPlayerID, v.Common().DartsInTurn)
	}
}

// Pulling the darts out after a bust passes the turn on without adding misses.
func TestBoardTakeoutAfterBust(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	g := x01Game(anna.PlayerID, guest.ID)
	g.Rules.(*x01.Settings).StartScore = 40
	g = withBoard(g, keller.ID)
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = s.AddDart(ctx, anna, v.Game.ID, throw.Dart{Number: 5, Segment: dartboard.Single})
	if v, err = s.AddDart(ctx, anna, v.Game.ID, t20); err != nil { // bust
		t.Fatal(err)
	}
	if !v.Common().WaitingForTakeout || v.Common().DartsInTurn != 2 {
		t.Fatalf("after bust: waiting=%v darts=%d, want waiting after 2 darts", v.Common().WaitingForTakeout, v.Common().DartsInTurn)
	}
	if v, err = s.BoardTakeout(ctx, keller); err != nil {
		t.Fatal(err)
	}
	if len(v.Darts) != 3 || !v.Darts[2].Takeout || *v.State.(x01.State).CurrentPlayerID != guest.ID {
		t.Errorf("after takeout: %d entries, current %d; want 2 darts, the takeout and the guest's turn", len(v.Darts), *v.State.(x01.State).CurrentPlayerID)
	}
}

func TestEditDart(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	g := x01Game(anna.PlayerID, guest.ID)
	g.Rules.(*x01.Settings).StartScore = 101
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	remaining := func(v GameView) uint64 { return v.State.(x01.State).Players[0].Remaining }
	single := func(n uint8) throw.Dart { return throw.Dart{Number: n, Segment: dartboard.Single} }

	// Anna throws T20, 1, 1: 39 left. Then the guest throws a 5.
	for _, d := range []throw.Dart{t20, single(1), single(1), single(5)} {
		if v, err = s.AddDart(ctx, anna, id, d); err != nil {
			t.Fatal(err)
		}
	}
	first, second := v.Darts[0].ID, v.Darts[1].ID

	// The second dart was a 19: 101 - 60 - 19 - 1 = 21 left.
	if v, err = s.EditDart(ctx, anna, id, second, single(19)); err != nil {
		t.Fatal(err)
	}
	if remaining(v) != 21 || v.Darts[1].Number != 19 || len(v.Darts) != 4 {
		t.Errorf("after correcting: %d left, darts %+v; want 21 left", remaining(v), v.Darts)
	}

	// The first dart was the bullseye: 101 - 50 - 19 - 1 = 31 left.
	if v, err = s.EditDart(ctx, anna, id, first, throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bullseye}); err != nil {
		t.Fatal(err)
	}
	if remaining(v) != 31 {
		t.Errorf("after second correction: %d left, want 31", remaining(v))
	}

	// A T20 as second dart overshoots (101 - 50 - 60 < 0): Anna's turn would end there and
	// her third dart would count for the guest. Refused, nothing changes.
	if _, err := s.EditDart(ctx, anna, id, second, t20); !errors.Is(err, ErrChangesThrowers) {
		t.Errorf("bust on the second dart: got %v, want ErrChangesThrowers", err)
	}
	if v, _ = s.Game(ctx, anna, id); remaining(v) != 31 {
		t.Errorf("refused correction changed the game: %d left", remaining(v))
	}

	if _, err := s.EditDart(ctx, anna, id, 999, t20); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown dart: got %v, want ErrNotFound", err)
	}
}

// There is no "abort": leaving a game with all your players ends it if fewer than two are
// left, which is all there is to aborting a game of one account.
func TestLeavingEndsAGameOfOneAccount(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	guest, _ := s.CreateGuest(ctx, anna, "Gast")

	older, err := createGame(ctx, s, anna, withBoard(x01Game(guest.ID), keller.ID), nil)
	if err != nil {
		t.Fatal(err)
	}
	v, err := createGame(ctx, s, anna, withBoard(x01Game(anna.PlayerID), keller.ID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acceptGame(ctx, s, ben, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); err != nil {
		t.Fatal(err)
	}

	// Ben alone with anna: either leaving ends the game.
	v, err = s.LeaveGame(ctx, ben, v.Game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Game.Aborted() || v.CanThrow || len(v.Darts) != 1 {
		t.Errorf("aborted game: aborted=%v can_throw=%v darts=%d, want aborted, no throwing, dart kept", v.Game.Aborted(), v.CanThrow, len(v.Darts))
	}
	if _, err := s.LeaveGame(ctx, anna, v.Game.ID); !errors.Is(err, ErrAborted) {
		t.Errorf("leave twice: got %v, want ErrAborted", err)
	}
	if _, err := s.AddDart(ctx, anna, v.Game.ID, t20); !errors.Is(err, ErrAborted) {
		t.Errorf("dart after abort: got %v, want ErrAborted", err)
	}
	if _, err := s.UndoDart(ctx, anna, v.Game.ID); !errors.Is(err, ErrAborted) {
		t.Errorf("undo after abort: got %v, want ErrAborted", err)
	}

	// The camera ignores the aborted game; the older one lost the board to it.
	if _, err := s.AddBoardDart(ctx, keller, t20); !errors.Is(err, ErrNoActiveGame) {
		t.Errorf("board dart after the abort: got %v, want ErrNoActiveGame", err)
	}
	if o, _ := s.Game(ctx, anna, older.Game.ID); o.HasBoard() {
		t.Error("the older game still has the board")
	}

	// A finished game can't be left.
	short := x01Game(anna.PlayerID)
	short.Rules.(*x01.Settings).StartScore = 40
	sv, _ := createGame(ctx, s, anna, short, nil)
	if _, err := s.AddDart(ctx, anna, sv.Game.ID, throw.Dart{Number: 20, Segment: dartboard.Double}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LeaveGame(ctx, anna, sv.Game.ID); !errors.Is(err, turn.ErrGameOver) {
		t.Errorf("leave a finished game: got %v, want turn.ErrGameOver", err)
	}

	// Leaving the lobby removes the game, and the invitation to it with it.
	lobby, _ := createGame(ctx, s, anna, x01Game(anna.PlayerID), []string{"ben"})
	if gv, err := s.LeaveGame(ctx, anna, lobby.Game.ID); err != nil || gv.Game.ID != 0 {
		t.Fatalf("leave the lobby: %v, %v", gv.Game.ID, err)
	}
	if invs, err := s.Invitations(ctx, ben); err != nil || len(invs) != 0 {
		t.Errorf("invitations after the creator left: got %d, %v, want none", len(invs), err)
	}
}

func TestDartPosition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	keller, _ := s.CreateBoard(ctx, anna, "Keller")
	g := x01Game(anna.PlayerID)
	g = withBoard(g, keller.ID)
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	x, y := 97.6, 21.0

	// The cameras' position is stored with the dart.
	seen := throw.Dart{Number: 13, Segment: dartboard.Outer, X: &x, Y: &y}
	if _, err := s.AddBoardDart(ctx, keller, seen); err != nil {
		t.Fatal(err)
	}
	// Darts entered by hand have none, even if one is given.
	if _, err := s.AddDart(ctx, anna, v.Game.ID, seen); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddBoardDart(ctx, keller, throw.Dart{Number: 1, Segment: dartboard.Outer, X: &x}); !errors.Is(err, ErrInvalid) {
		t.Errorf("only x: got %v, want ErrInvalid", err)
	}

	v, _ = s.Game(ctx, anna, v.Game.ID)
	camera, hand := v.Darts[0], v.Darts[1]
	if camera.X == nil || *camera.X != x || *camera.Y != y {
		t.Errorf("camera dart position = %v,%v, want %v,%v", camera.X, camera.Y, x, y)
	}
	if hand.X != nil || hand.Y != nil {
		t.Errorf("hand-entered dart has a position")
	}

	// A correction changes the field and keeps where the cameras saw the dart.
	v, err = s.EditDart(ctx, anna, v.Game.ID, camera.ID, throw.Dart{Number: 13, Segment: dartboard.Triple})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Darts[0]; got.Segment != dartboard.Triple || got.X == nil || *got.X != x {
		t.Errorf("corrected dart = %+v, want T13 at the cameras' position", got)
	}
}

func TestDecideBullOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	papa, _ := s.CreateGuest(ctx, anna, "Papa")
	g := x01Game(anna.PlayerID, papa.ID)
	g.BullOff = true
	v, err := createGame(ctx, s, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.DecideBullOff(ctx, anna, v.Game.ID, papa.ID); !errors.Is(err, ErrInvalid) {
		t.Errorf("decide without a tie: got %v, want ErrInvalid", err)
	}
	bull := throw.Dart{Number: dartboard.BullNumber, Segment: dartboard.Bull}
	for range 2 {
		if _, err := s.AddDart(ctx, anna, v.Game.ID, bull); err != nil {
			t.Fatal(err)
		}
	}
	v, err = s.DecideBullOff(ctx, anna, v.Game.ID, papa.ID)
	if err != nil {
		t.Fatal(err)
	}
	st := v.State.(x01.State)
	if st.BullingOff || *st.CurrentPlayerID != papa.ID {
		t.Errorf("bulling off=%v current=%d, want Papa to start", st.BullingOff, *st.CurrentPlayerID)
	}
}

func TestSetColor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")

	p, err := s.Player(ctx, anna)
	if err != nil || p.Color != "" {
		t.Fatalf("new player: color %q, %v; want none", p.Color, err)
	}
	if err := s.SetColor(ctx, anna, "pink"); err != nil {
		t.Fatal(err)
	}
	if p, _ = s.Player(ctx, anna); p.Color != "pink" || p.Accent() != "pink" {
		t.Errorf("color = %q, accent %q, want pink", p.Color, p.Accent())
	}
	if err := s.SetColor(ctx, anna, "plaid"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown color: got %v, want ErrInvalid", err)
	}
	if err := s.SetColor(ctx, anna, ""); err != nil {
		t.Fatal(err)
	}
	if p, _ = s.Player(ctx, anna); p.Color != "" {
		t.Errorf("after reset: color %q, want none", p.Color)
	}
}

func TestOnlyOwnDartsAreCorrected(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)
	papa, _ := s.CreateGuest(ctx, anna, "Papa")

	v, err := createGame(ctx, s, anna, x01Game(anna.PlayerID, papa.ID), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	if _, err := acceptGame(ctx, s, ben, id, nil); err != nil {
		t.Fatal(err)
	}
	for range 3 { // anna's visit, then papa's, so ben is up
		_, _ = s.AddDart(ctx, anna, id, t20)
	}
	for range 3 {
		_, _ = s.AddDart(ctx, anna, id, t20) // her guest's
	}
	if v, err = s.AddDart(ctx, ben, id, throw.Dart{Number: 5, Segment: dartboard.Single}); err != nil {
		t.Fatal(err)
	}
	annas, bens, papas := v.Darts[0], v.Darts[6], v.Darts[3]

	fix := throw.Dart{Number: 1, Segment: dartboard.Single}
	if _, err := s.EditDart(ctx, ben, id, annas.ID, fix); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben corrects anna's dart: got %v, want ErrForbidden", err)
	}
	if _, err := s.EditDart(ctx, anna, id, bens.ID, fix); !errors.Is(err, ErrForbidden) {
		t.Errorf("anna corrects ben's dart: got %v, want ErrForbidden", err)
	}
	if _, err := s.EditDart(ctx, anna, id, papas.ID, fix); err != nil {
		t.Errorf("anna corrects her guest's dart: %v", err)
	}
	if _, err := s.EditDart(ctx, anna, id, annas.ID, fix); err != nil {
		t.Errorf("anna corrects her own dart: %v", err)
	}
	if got, _ := s.Game(ctx, ben, id); !got.MayEdit(6) || got.MayEdit(0) || !got.MayUndo() {
		t.Errorf("ben may edit his dart %v, anna's %v, undo %v; want true, false, true", got.MayEdit(6), got.MayEdit(0), got.MayUndo())
	}
}

// isError reports whether err wraps an error of type E.
func isError[E error](err error) bool {
	_, ok := errors.AsType[E](err)
	return ok
}

func TestChangePassword(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	here, _, err := s.Login(ctx, "anna", "password")
	if err != nil {
		t.Fatal(err)
	}
	elsewhere, _, _ := s.Login(ctx, "anna", "password")

	if err := s.ChangePassword(ctx, anna, "wrong", "new password", here); !errors.Is(err, ErrWrongPassword) {
		t.Errorf("wrong current password: got %v, want ErrWrongPassword", err)
	}
	if err := s.ChangePassword(ctx, anna, "password", "short", here); !errors.Is(err, account.ErrInvalidPassword) {
		t.Errorf("short password: got %v, want ErrInvalidPassword", err)
	}
	if err := s.ChangePassword(ctx, anna, "password", "new password", here); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Authenticate(ctx, here); err != nil {
		t.Errorf("the session that changed it: %v, want it logged in", err)
	}
	if _, err := s.Authenticate(ctx, elsewhere); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("the other session: got %v, want ErrUnauthenticated", err)
	}
	if _, _, err := s.Login(ctx, "anna", "password"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("login with the old password: got %v, want ErrUnauthenticated", err)
	}
	if _, _, err := s.Login(ctx, "anna", "new password"); err != nil {
		t.Errorf("login with the new password: %v", err)
	}
}

func TestFriendLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	others := make([]account.Account, MaxFriends)
	for i := range others {
		others[i] = register(t, s, fmt.Sprintf("user%d", i))
	}
	register(t, s, "ben")
	cara := register(t, s, "cara")

	// Half of them friends, the others asked: all but the last count.
	for i, o := range others[:MaxFriends-1] {
		if i%2 == 0 {
			befriend(t, s, anna, o)
		} else if err := s.RequestFriend(ctx, anna, o.Username); err != nil {
			t.Fatal(err)
		}
	}
	// Requests anna received don't count.
	if err := s.RequestFriend(ctx, cara, "anna"); err != nil {
		t.Fatal(err)
	}
	last := others[MaxFriends-1]
	if err := s.RequestFriend(ctx, anna, last.Username); err != nil {
		t.Fatalf("the last request: %v", err)
	}

	if err := s.RequestFriend(ctx, anna, "ben"); !errors.Is(err, ErrTooManyFriends) {
		t.Errorf("one request too many: got %v, want ErrTooManyFriends", err)
	}
	if err := s.AcceptFriend(ctx, anna, cara.ID); !errors.Is(err, ErrTooManyFriends) {
		t.Errorf("accepting one too many: got %v, want ErrTooManyFriends", err)
	}
	if err := s.RequestFriend(ctx, anna, "cara"); !errors.Is(err, ErrTooManyFriends) {
		t.Errorf("asking back one too many: got %v, want ErrTooManyFriends", err)
	}
	// The limit is the accepting side's: the last one has room for anna.
	if err := s.AcceptFriend(ctx, last, anna.ID); err != nil {
		t.Errorf("accepting anna's request: %v", err)
	}

	if err := s.RemoveFriend(ctx, anna, last.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.AcceptFriend(ctx, anna, cara.ID); err != nil {
		t.Errorf("accepting after removing a friend: %v", err)
	}
}

func TestGuestLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	var first int64
	for i := range MaxGuests {
		g, err := s.CreateGuest(ctx, anna, fmt.Sprintf("Guest %d", i))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = g.ID
		}
	}
	if _, err := s.CreateGuest(ctx, anna, "One more"); !errors.Is(err, ErrTooManyGuests) {
		t.Errorf("one guest too many: got %v, want ErrTooManyGuests", err)
	}
	if err := s.DeleteGuest(ctx, anna, first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateGuest(ctx, anna, "One more"); err != nil {
		t.Errorf("a guest after deleting one: %v", err)
	}
}
