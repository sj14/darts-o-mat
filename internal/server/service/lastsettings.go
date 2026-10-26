package service

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
)

// DefaultSettings are the settings the start form shows for a mode the account hasn't
// created a game of yet: one leg without sets, bull-off and time limit, X01 from 501 with
// a double to check out, around the clock with the bull at the end.
func DefaultSettings(mode game.Mode) game.Settings {
	s := game.Settings{Mode: mode, Legs: 1, Sets: 1, Rules: game.NewRules(mode)}
	switch rules := s.Rules.(type) {
	case *x01.Settings:
		rules.StartScore, rules.OutMode = 501, x01.OutModeDouble
	case *aroundtheclock.Settings:
		rules.Bull = true
	}
	return s
}

// NewSettings returns the settings the start form shows for a new game or tournament of
// the mode: those of the account's last one (CreateGame and CreateTournament remember
// them, a quick match doesn't), else DefaultSettings.
func (s *Service) NewSettings(ctx context.Context, acc account.Account, mode game.Mode) (game.Settings, error) {
	last, ok, err := s.store.LastSettings(ctx, acc.ID, mode)
	if err != nil || !ok {
		return DefaultSettings(mode), err
	}
	return last, nil
}
