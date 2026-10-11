package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/game/x01"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

func newService(t *testing.T) *Service {
	t.Helper()
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	return New(st, Options{Registration: true})
}

func register(t *testing.T, s *Service, username string) account.Account {
	t.Helper()
	a, err := s.Register(context.Background(), username, "", "password")
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
	return game.Game{
		Mode:      game.ModeX01,
		PlayerIDs: playerIDs,
		X01:       &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble},
	}
}

var t20 = throw.Dart{Number: 20, Multiplier: 3}

func TestAuth(t *testing.T) {
	ctx := context.Background()
	s := newService(t)

	a := register(t, s, " Anna ")
	if a.Username != "anna" {
		t.Errorf("username = %q, want it normalized to anna", a.Username)
	}
	if _, err := s.Register(ctx, "ANNA", "", "password"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplicate username: got %v, want ErrUsernameTaken", err)
	}
	if _, err := s.Register(ctx, "ben", "", "short"); !errors.Is(err, ErrInvalid) {
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

	closed := New(s.store, Options{Registration: false})
	if _, err := closed.Register(ctx, "cara", "", "password"); !errors.Is(err, ErrRegistrationClosed) {
		t.Errorf("closed registration: got %v, want ErrRegistrationClosed", err)
	}
}

func TestGuestsBelongToTheirAccount(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	guest, err := s.CreateGuest(ctx, anna, "Papa")
	if err != nil {
		t.Fatal(err)
	}

	players, _ := s.Players(ctx, anna)
	if len(players) != 2 || players[0].ID != anna.PlayerID || players[1].ID != guest.ID {
		t.Errorf("anna's players = %+v, want herself followed by her guest", players)
	}

	if _, err := s.CreateGame(ctx, ben, x01Game(ben.PlayerID, guest.ID), nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben uses anna's guest: got %v, want ErrInvalid", err)
	}
	if _, err := s.CreateGame(ctx, ben, x01Game(anna.PlayerID), nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben adds anna without invitation: got %v, want ErrInvalid", err)
	}
	var unknown UnknownUserError
	if _, err := s.CreateGame(ctx, ben, x01Game(ben.PlayerID), []string{"nobody"}); !errors.As(err, &unknown) || !errors.Is(err, ErrInvalid) {
		t.Errorf("invite unknown user: got %v, want UnknownUserError wrapping ErrInvalid", err)
	}
	if _, err := s.CreateGame(ctx, ben, x01Game(), []string{"Ben"}); !errors.Is(err, ErrInviteSelf) {
		t.Errorf("invite yourself: got %v, want ErrInviteSelf", err)
	}
}

func TestInvitationFlow(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")
	befriend(t, s, anna, ben)

	v, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID, guest.ID), []string{"Ben"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID
	if want := []int64{anna.PlayerID, guest.ID, ben.PlayerID}; !slices.Equal(v.Game.PlayerIDs, want) {
		t.Errorf("player order = %v, want %v", v.Game.PlayerIDs, want)
	}
	if v.Ready {
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

	v, err = s.AcceptInvitation(ctx, ben, id)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Ready || !v.CanThrow {
		t.Errorf("after accepting: ready=%v canThrow=%v, want both", v.Ready, v.CanThrow)
	}
	if _, err := s.AcceptInvitation(ctx, ben, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("accepting twice: got %v, want ErrNotFound", err)
	}

	if _, err := s.AddDart(ctx, anna, id, t20); err != nil {
		t.Errorf("dart by owner: %v", err)
	}
	if _, err := s.AddDart(ctx, ben, id, t20); err != nil {
		t.Errorf("dart by accepted invitee: %v", err)
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
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")
	befriend(t, s, anna, ben)
	befriend(t, s, cara, anna)

	v, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben", "cara"})
	if err != nil {
		t.Fatal(err)
	}
	id := v.Game.ID

	if _, err := s.WithdrawInvitation(ctx, ben, id, cara.PlayerID); !errors.Is(err, ErrForbidden) {
		t.Errorf("withdraw by non-owner: got %v, want ErrForbidden", err)
	}

	if err := s.DeclineInvitation(ctx, ben, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Game(ctx, ben, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben after declining: got %v, want ErrNotFound", err)
	}

	v, err = s.WithdrawInvitation(ctx, anna, id, cara.PlayerID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(v.Game.PlayerIDs, []int64{anna.PlayerID}) || !v.Ready {
		t.Errorf("after withdrawing: players=%v ready=%v, want only anna and ready", v.Game.PlayerIDs, v.Ready)
	}
}

func TestGameWithoutPlayersLeftIsDeleted(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")
	befriend(t, s, anna, ben)

	// Anna only hosts, she doesn't play herself.
	v, err := s.CreateGame(ctx, anna, x01Game(), []string{"ben"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeclineInvitation(ctx, ben, v.Game.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Game(ctx, anna, v.Game.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty game: got %v, want ErrNotFound", err)
	}
}

func TestBoards(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben")

	b, err := s.CreateBoard(ctx, anna, "Keller")
	if err != nil {
		t.Fatal(err)
	}

	g := x01Game(ben.PlayerID)
	g.BoardID = &b.ID
	if _, err := s.CreateGame(ctx, ben, g, nil); !errors.Is(err, ErrInvalid) {
		t.Errorf("ben uses anna's board: got %v, want ErrInvalid", err)
	}
	if err := s.DeleteBoard(ctx, ben, b.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben deletes anna's board: got %v, want ErrNotFound", err)
	}

	g.PlayerIDs = []int64{anna.PlayerID}
	if _, err := s.CreateGame(ctx, anna, g, nil); err != nil {
		t.Errorf("anna uses her board: %v", err)
	}
}

func TestFriends(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	anna, ben, cara := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara")

	var notFriend NotFriendError
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben"}); !errors.As(err, &notFriend) {
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
	var unknown UnknownUserError
	if err := s.RequestFriend(ctx, anna, "nobody"); !errors.As(err, &unknown) {
		t.Errorf("unknown user: got %v, want UnknownUserError", err)
	}

	annas, _ := s.Friends(ctx, anna)
	bens, _ := s.Friends(ctx, ben)
	if len(annas.Outgoing) != 1 || annas.Outgoing[0].Username != "ben" || len(bens.Incoming) != 1 || bens.Incoming[0].Username != "anna" {
		t.Fatalf("pending request: anna %+v, ben %+v", annas, bens)
	}
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben"}); !errors.As(err, &notFriend) {
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
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"ben"}); err != nil {
		t.Errorf("invite friend: %v", err)
	}
	if _, err := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), []string{"cara"}); !errors.As(err, &notFriend) {
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
	ctx := context.Background()
	s := newService(t)
	anna := register(t, s, "anna")
	guest, _ := s.CreateGuest(ctx, anna, "Papa")

	g := x01Game(anna.PlayerID, guest.ID)
	g.X01.StartScore = 40
	v, err := s.CreateGame(ctx, anna, g, nil)
	if err != nil {
		t.Fatal(err)
	}
	miss := throw.Dart{Miss: true}
	for _, d := range []throw.Dart{{Number: 20, Multiplier: 2}, miss, miss, miss} {
		if _, err := s.AddDart(ctx, anna, v.Game.ID, d); err != nil {
			t.Fatal(err)
		}
	}

	// A running game doesn't count.
	running, _ := s.CreateGame(ctx, anna, x01Game(anna.PlayerID), nil)
	if _, err := s.AddDart(ctx, anna, running.Game.ID, t20); err != nil {
		t.Fatal(err)
	}

	summaries, err := s.Stats(ctx, anna)
	if err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 {
		t.Fatalf("got %d summaries, want anna and her guest", len(summaries))
	}
	if got := summaries[0].X01; got.Games != 1 || got.Wins != 1 || got.Darts != 1 || got.Average != 120 || got.HighestCheckout != 40 || got.BestGame != 1 {
		t.Errorf("anna's stats = %+v", got)
	}
	if got := summaries[1].X01; got.Games != 1 || got.Wins != 0 || got.Darts != 3 || got.Average != 0 || got.BestGame != 0 {
		t.Errorf("guest's stats = %+v", got)
	}
}
