package service

import (
	"context"
	"errors"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
)

// Every admin may make other accounts admins and take the role from any admin, themselves
// included, except from the last one.
func TestSetAdmin(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben") // anna is the admin

	admins := func() []string {
		t.Helper()
		list, err := s.store.ListAdmins(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, a := range list {
			names = append(names, a.Username)
		}
		return names
	}
	// reload reads the account again, as the next request would.
	reload := func(name string) account.Account {
		t.Helper()
		a, err := s.store.GetAccountByUsername(ctx, name)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}

	if err := s.SetAdmin(ctx, ben, "ben", true); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben makes himself admin: %v, want ErrForbidden", err)
	}
	if _, err := s.Admins(ctx, ben); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben lists the admins: %v, want ErrForbidden", err)
	}
	if err := s.SetAdmin(ctx, anna, "nobody", true); !errors.As(err, new(UnknownUserError)) {
		t.Errorf("unknown username: %v, want UnknownUserError", err)
	}
	if err := s.SetAdmin(ctx, anna, "anna", false); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("the last admin gives up the role: %v, want ErrLastAdmin", err)
	}

	if err := s.SetAdmin(ctx, anna, " BEN ", true); err != nil {
		t.Fatal(err)
	}
	if got := admins(); len(got) != 2 || got[0] != "anna" || got[1] != "ben" {
		t.Fatalf("admins = %q, want anna and ben", got)
	}
	// Ben, now an admin, takes the role from anna, who made him one.
	if err := s.SetAdmin(ctx, reload("ben"), "anna", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAdmin(ctx, reload("anna"), "anna", true); !errors.Is(err, ErrForbidden) {
		t.Errorf("anna, no longer an admin: %v, want ErrForbidden", err)
	}
	if err := s.SetAdmin(ctx, reload("ben"), "ben", false); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("ben, now the last admin, gives up the role: %v, want ErrLastAdmin", err)
	}
	// Anna can delete her account now.
	if err := s.DeleteAccount(ctx, reload("anna"), "password"); err != nil {
		t.Errorf("deleting an account that is no longer an admin: %v", err)
	}
	if got := admins(); len(got) != 1 || got[0] != "ben" {
		t.Errorf("admins = %q, want ben", got)
	}
}
