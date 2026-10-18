package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLoginLimit(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	register(t, s, "anna")
	register(t, s, "ben")

	// Without a limit nothing is refused.
	for range 20 {
		if _, _, err := s.Login(ctx, "anna", "wrong-password"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("without a limit: got %v, want ErrUnauthenticated", err)
		}
	}

	s.limitLogins(3)
	offset := time.Duration(0)
	s.now = func() time.Time { return time.Now().Add(offset) }

	for range 3 {
		if _, _, err := s.Login(ctx, "Anna", "wrong-password"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("failed login: got %v, want ErrUnauthenticated", err)
		}
	}
	// Now even the right password is refused, in any spelling; others aren't affected.
	if _, _, err := s.Login(ctx, " ANNA", "password"); !errors.Is(err, ErrTooManyLogins) {
		t.Errorf("right password while blocked: got %v, want ErrTooManyLogins", err)
	}
	if _, _, err := s.Login(ctx, "ben", "password"); err != nil {
		t.Errorf("another username: %v", err)
	}
	// Names that don't exist are limited like the others: nothing shows whether they do.
	for range 3 {
		_, _, _ = s.Login(ctx, "nobody", "x")
	}
	if _, _, err := s.Login(ctx, "nobody", "x"); !errors.Is(err, ErrTooManyLogins) {
		t.Errorf("unknown name: got %v, want ErrTooManyLogins", err)
	}
	// Trying on doesn't extend the block: it ends 15 minutes after the last counted failure.
	offset = loginWindow - time.Minute
	_, _, _ = s.Login(ctx, "anna", "password")
	offset = loginWindow + time.Second
	if _, _, err := s.Login(ctx, "anna", "password"); err != nil {
		t.Errorf("after the window: %v", err)
	}

	// A login that works forgets the failures.
	for range 2 {
		_, _, _ = s.Login(ctx, "ben", "wrong-password")
	}
	if _, _, err := s.Login(ctx, "ben", "password"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		_, _, _ = s.Login(ctx, "ben", "wrong-password")
	}
	if _, _, err := s.Login(ctx, "ben", "password"); err != nil {
		t.Errorf("two failures after a success: %v", err)
	}
}

func TestLoginLimitPerNetwork(t *testing.T) {
	ctx := context.Background()
	s := newService(t)
	register(t, s, "anna")
	s.limitLogins(2) // a network may fail 10 times

	// One address guessing at many usernames is stopped, though each name failed once.
	home := WithNetwork(ctx, "203.0.113.7/32")
	for i := range 10 {
		if _, _, err := s.Login(home, "user"+string(rune('a'+i)), "x"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("attempt %d: got %v, want ErrUnauthenticated", i+1, err)
		}
	}
	if _, _, err := s.Login(home, "anna", "password"); !errors.Is(err, ErrTooManyLogins) {
		t.Errorf("the right login from the blocked network: got %v, want ErrTooManyLogins", err)
	}
	// Other networks and logins without a known network aren't affected.
	if _, _, err := s.Login(WithNetwork(ctx, "2001:db8:1:2::/64"), "anna", "password"); err != nil {
		t.Errorf("another network: %v", err)
	}
	if _, _, err := s.Login(ctx, "anna", "password"); err != nil {
		t.Errorf("unknown network: %v", err)
	}
}

func TestClientNetwork(t *testing.T) {
	s := newService(t)
	none := func(string) string { return "" }
	for remote, want := range map[string]string{
		"203.0.113.7:51234":          "203.0.113.7/32",
		"[2001:db8:1:2:aaaa::5]:443": "2001:db8:1:2::/64",
		"[::ffff:203.0.113.7]:80":    "203.0.113.7/32",
		"127.0.0.1:1234":             "", // a proxy on the same machine
		"10.1.2.3:1234":              "", // a pod or container network
		"192.168.1.20:1234":          "",
		"100.64.1.1:1234":            "", // shared address space
		"[fe80::1]:1234":             "",
		"[fd00::1]:1234":             "",
		"garbage":                    "",
	} {
		if got := s.ClientNetwork(remote, none); got != want {
			t.Errorf("%s: got %q, want %q", remote, got, want)
		}
	}

	// With a header the proxy's address doesn't matter: its last value counts, the ones
	// before were sent by the client.
	s.opts.ClientIPHeader = "X-Forwarded-For"
	header := func(v string) func(string) string {
		return func(name string) string {
			if name != "X-Forwarded-For" {
				t.Errorf("read the header %q", name)
			}
			return v
		}
	}
	for value, want := range map[string]string{
		"203.0.113.7":                  "203.0.113.7/32",
		"198.51.100.1, 203.0.113.7":    "203.0.113.7/32",
		"1.2.3.4,  2001:db8:1:2:3::9 ": "2001:db8:1:2::/64",
		"10.0.0.8":                     "10.0.0.8/32", // the proxy says so
		"":                             "",
		"not an address":               "",
	} {
		if got := s.ClientNetwork("10.0.0.1:80", header(value)); got != want {
			t.Errorf("header %q: got %q, want %q", value, got, want)
		}
	}
}
