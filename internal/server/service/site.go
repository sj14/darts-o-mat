package service

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
)

// RegistrationOpen says whether anyone may create an account. The setting is read from the
// database every time, so every replica follows what an admin saved.
func (s *Service) RegistrationOpen(ctx context.Context) (bool, error) {
	settings, err := s.store.SiteSettings(ctx)
	return settings.Registration, err
}

// SetRegistration allows or forbids creating accounts. Only an admin may.
func (s *Service) SetRegistration(ctx context.Context, acc account.Account, open bool) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	settings, err := s.store.SiteSettings(ctx)
	if err != nil {
		return err
	}
	settings.Registration = open
	return s.store.SetSiteSettings(ctx, settings)
}
