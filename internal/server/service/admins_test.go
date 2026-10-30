package service

import (
	"context"
	"errors"
	"strings"
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
	if err := s.SetAdmin(ctx, anna, "anna", false); !errors.Is(err, account.ErrLastAdmin) {
		t.Errorf("the last admin gives up the role: %v, want account.ErrLastAdmin", err)
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
	if err := s.SetAdmin(ctx, reload("ben"), "ben", false); !errors.Is(err, account.ErrLastAdmin) {
		t.Errorf("ben, now the last admin, gives up the role: %v, want account.ErrLastAdmin", err)
	}
	// Anna can delete her account now.
	if err := s.DeleteAccount(ctx, reload("anna"), "password"); err != nil {
		t.Errorf("deleting an account that is no longer an admin: %v", err)
	}
	if got := admins(); len(got) != 1 || got[0] != "ben" {
		t.Errorf("admins = %q, want ben", got)
	}
}

// The admin lists and searches the accounts, disables one, so it can't log in and its
// sessions end, enables it again and deletes it; admins can be neither disabled nor deleted.
func TestManageAccounts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newService(t)
	anna, ben := register(t, s, "anna"), register(t, s, "ben") // anna is the admin
	if _, err := s.Register(ctx, "cara_x", "cara@example.com", "", "password"); err != nil {
		t.Fatal(err)
	}

	names := func(search string) string {
		t.Helper()
		accounts, more, err := s.Accounts(ctx, anna, search)
		if err != nil || more {
			t.Fatalf("accounts %q: more %v, %v", search, more, err)
		}
		var names []string
		for _, a := range accounts {
			names = append(names, a.Username)
		}
		return strings.Join(names, " ")
	}
	if got, want := names(""), "cara_x ben anna"; got != want {
		t.Errorf("all accounts: %s, want the newest first: %s", got, want)
	}
	if got, want := names(" EXAMPLE.com"), "cara_x"; got != want {
		t.Errorf("search by email: %s, want %s", got, want)
	}
	if got, want := names("a_"), "cara_x"; got != want {
		t.Errorf("search with _: %s, want only the name that has it: %s", got, want)
	}
	if _, _, err := s.Accounts(ctx, ben, ""); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben lists the accounts: %v, want ErrForbidden", err)
	}

	token, _, err := s.Login(ctx, "ben", "password")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDisabled(ctx, ben, anna.ID, true); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben disables anna: %v, want ErrForbidden", err)
	}
	if err := s.SetDisabled(ctx, anna, anna.ID, true); !errors.Is(err, account.ErrAdminAccount) {
		t.Errorf("disabling an admin: %v, want account.ErrAdminAccount", err)
	}
	if err := s.SetDisabled(ctx, anna, ben.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("ben's session after disabling: %v, want ErrUnauthenticated", err)
	}
	if _, _, err := s.Login(ctx, "ben", "password"); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("disabled ben logs in: %v, want ErrAccountDisabled", err)
	}
	if _, _, err := s.Login(ctx, "ben", "wrong-password"); !errors.Is(err, ErrUnauthenticated) {
		t.Errorf("disabled ben with a wrong password: %v, want ErrUnauthenticated, which doesn't tell", err)
	}
	if err := s.SetAdmin(ctx, anna, "ben", true); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("making disabled ben an admin: %v, want ErrAccountDisabled", err)
	}

	if err := s.SetDisabled(ctx, anna, ben.ID, false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Login(ctx, "ben", "password"); err != nil {
		t.Errorf("enabled ben logs in: %v", err)
	}

	if err := s.DeleteOtherAccount(ctx, ben, anna.ID); !errors.Is(err, ErrForbidden) {
		t.Errorf("ben deletes anna: %v, want ErrForbidden", err)
	}
	if err := s.DeleteOtherAccount(ctx, anna, anna.ID); !errors.Is(err, account.ErrAdminAccount) {
		t.Errorf("deleting an admin: %v, want account.ErrAdminAccount", err)
	}
	if err := s.DeleteOtherAccount(ctx, anna, ben.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.GetAccount(ctx, ben.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("ben after deleting: %v, want ErrNotFound", err)
	}
	if err := s.SetDisabled(ctx, anna, ben.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("disabling a deleted account: %v, want ErrNotFound", err)
	}
}
