package service

import (
	"context"
	"errors"
	"strings"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

// MaxAccountsShown is how many accounts the admin page lists at most, the newest first; a
// search finds the others.
const MaxAccountsShown = 50

// Accounts returns up to MaxAccountsShown accounts for the admin, the newest first, with a
// search only those whose username or email contains it, and whether there are more.
// ErrForbidden for everyone but an admin.
func (s *Service) Accounts(ctx context.Context, acc account.Account, search string) ([]account.Account, bool, error) {
	if err := checkAdmin(acc); err != nil {
		return nil, false, err
	}
	search = strings.ToLower(strings.TrimSpace(search))
	accounts, err := s.store.ListAccounts(ctx, search, MaxAccountsShown+1)
	if err != nil || len(accounts) <= MaxAccountsShown {
		return accounts, false, err
	}
	return accounts[:MaxAccountsShown], true, nil
}

// SetDisabled disables the account with the ID, so it can't log in anymore and its sessions
// end, or enables it again. Its games, boards and friends stay. An admin can't be disabled
// (account.ErrAdminAccount). Only an admin may.
func (s *Service) SetDisabled(ctx context.Context, acc account.Account, id int64, disabled bool) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	if _, err := s.store.GetAccount(ctx, id); err != nil {
		return err
	}
	return s.store.SetDisabled(ctx, id, disabled)
}

// DeleteOtherAccount deletes the account with the ID like the account itself would (see
// DeleteAccount), without its password. An admin can't be deleted this way
// (account.ErrAdminAccount), also not the own account. Only an admin may.
func (s *Service) DeleteOtherAccount(ctx context.Context, acc account.Account, id int64) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	other, err := s.store.GetAccount(ctx, id)
	if err != nil {
		return err
	}
	if other.Admin {
		return account.ErrAdminAccount
	}
	return s.deleteAccount(ctx, other)
}

// Admins returns the admins of the server, by username; ErrForbidden for everyone but an
// admin.
func (s *Service) Admins(ctx context.Context, acc account.Account) ([]account.Account, error) {
	if err := checkAdmin(acc); err != nil {
		return nil, err
	}
	return s.store.ListAdmins(ctx)
}

// SetAdmin gives the account with the username the admin role or takes it away; every
// admin may do both, also to another admin and to themselves. The last admin can't give the
// role up (account.ErrLastAdmin, made sure of in the store's transaction), so the server is
// never left without one.
func (s *Service) SetAdmin(ctx context.Context, acc account.Account, username string, admin bool) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	username = account.NormalizeUsername(username)
	other, err := s.store.GetAccountByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return UnknownUserError{Username: username}
	}
	if err != nil {
		return err
	}
	if other.Admin == admin {
		return nil
	}
	// Admins can't be disabled, so a disabled account doesn't become one.
	if admin && other.Disabled {
		return ErrAccountDisabled
	}
	return s.store.SetAdmin(ctx, other.ID, admin)
}
