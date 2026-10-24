package service

import (
	"context"
	"errors"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

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
// role up (ErrLastAdmin, made sure of in the store's transaction), so the server is never
// left without one.
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
	return s.store.SetAdmin(ctx, other.ID, admin)
}
