package service

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/site"
)

// Site returns the admin's settings of the server. They are read from the database once and
// then kept in memory, like the pages.
func (s *Service) Site(ctx context.Context) (site.Settings, error) {
	s.siteMu.Lock()
	defer s.siteMu.Unlock()
	if s.site != nil {
		return *s.site, nil
	}
	settings, err := s.store.SiteSettings(ctx)
	if err != nil {
		return site.Settings{}, err
	}
	s.site = &settings
	return settings, nil
}

// RegistrationOpen says whether anyone may create an account.
func (s *Service) RegistrationOpen(ctx context.Context) (bool, error) {
	settings, err := s.Site(ctx)
	return settings.Registration, err
}

// SetRegistration allows or forbids creating accounts. Only the admin may.
func (s *Service) SetRegistration(ctx context.Context, acc account.Account, open bool) error {
	if err := checkAdmin(acc); err != nil {
		return err
	}
	settings, err := s.Site(ctx)
	if err != nil {
		return err
	}
	settings.Registration = open

	s.siteMu.Lock()
	defer s.siteMu.Unlock()
	if err := s.store.SetSiteSettings(ctx, settings); err != nil {
		return err
	}
	s.site = &settings
	return nil
}
