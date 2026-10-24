package service

import (
	"context"
	"testing"
	"time"
)

func TestPresence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	now := time.Now()
	s.now = func() time.Time { return now }
	anna, ben, cara, dora := register(t, s, "anna"), register(t, s, "ben"), register(t, s, "cara"), register(t, s, "dora")
	befriend(t, s, anna, ben)
	befriend(t, s, anna, cara)
	befriend(t, s, ben, cara)
	if err := s.RequestFriend(ctx, dora, "anna"); err != nil {
		t.Fatal(err)
	}

	// presences returns what anna sees of her friends, by username.
	presences := func() map[string]Presence {
		t.Helper()
		list, err := s.Friends(ctx, anna)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range list.Incoming {
			if f.Presence != "" {
				t.Errorf("request of %s shows presence %q", f.Username, f.Presence)
			}
		}
		got := map[string]Presence{}
		for _, f := range list.Friends {
			got[f.Username] = f.Presence
		}
		return got
	}
	check := func(want map[string]Presence) {
		t.Helper()
		got := presences()
		for name, p := range want {
			if got[name] != p {
				t.Errorf("%s: got %q, want %q", name, got[name], p)
			}
		}
	}

	check(map[string]Presence{"ben": PresenceOffline, "cara": PresenceOffline})

	s.seen(ben.ID)
	check(map[string]Presence{"ben": PresenceOnline, "cara": PresenceOffline})

	v, err := createGame(ctx, s, ben, x01Game(ben.PlayerID), []string{"cara"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acceptGame(ctx, s, cara, v.Game.ID, nil); err != nil {
		t.Fatal(err)
	}
	// cara plays too, but only a page with a live stream says she is there.
	check(map[string]Presence{"ben": PresencePlaying, "cara": PresenceOffline})
	stop := s.Watching(cara)
	check(map[string]Presence{"ben": PresencePlaying, "cara": PresencePlaying})

	ben, err = s.SetInvisible(ctx, ben, true)
	if err != nil {
		t.Fatal(err)
	}
	check(map[string]Presence{"ben": PresenceOffline, "cara": PresencePlaying})

	now = now.Add(time.Hour) // the stream keeps cara online however long it is open
	s.seen(ben.ID)
	check(map[string]Presence{"ben": PresenceOffline, "cara": PresencePlaying})
	stop()
	now = now.Add(onlineFor - time.Second)
	check(map[string]Presence{"cara": PresencePlaying})
	now = now.Add(time.Second)
	check(map[string]Presence{"cara": PresenceOffline})

	if _, err := s.SetInvisible(ctx, ben, false); err != nil {
		t.Fatal(err)
	}
	s.seen(ben.ID)
	check(map[string]Presence{"ben": PresencePlaying})
}
