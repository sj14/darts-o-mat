package service

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/stats"
)

// Stats returns the statistics of the account's player and its guests over all finished games.
func (s *Service) Stats(ctx context.Context, acc account.Account) ([]stats.Summary, error) {
	players, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	summaries := make([]stats.Summary, len(players))
	for i, p := range players {
		summaries[i].Player = p
	}

	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return nil, err
	}
	for _, g := range games {
		darts, err := s.store.ListDarts(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		state, err := g.Play(darts)
		if err != nil {
			return nil, err
		}
		for i := range summaries {
			summaries[i].Add(g, state)
		}
	}
	return summaries, nil
}
