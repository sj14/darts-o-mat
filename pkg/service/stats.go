package service

import (
	"context"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/stats"
)

// HeatDarts is how many of a player's most recent camera darts the heatmap shows.
const HeatDarts = 5000

// Stats returns the statistics of the account's player and its guests over all completed
// games and finished tournaments, and where their darts hit the board.
//
// Once a game is completed nothing about it changes anymore, so its result per player
// is stored (stats.Result) the first time it is seen here and added up from then on;
// only games that aren't stored yet are replayed. Games that are over but not completed
// (a dart could still be corrected) don't count yet.
func (s *Service) Stats(ctx context.Context, acc account.Account) ([]stats.Summary, error) {
	players, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	summaries := make([]stats.Summary, len(players))
	ids := make([]int64, len(players))
	for i, p := range players {
		summaries[i].Player = p
		ids[i] = p.ID
		points, err := s.store.ListPositions(ctx, p.ID, HeatDarts)
		if err != nil {
			return nil, err
		}
		summaries[i].Heat = stats.NewHeat(points)
	}

	results, err := s.store.ListResults(ctx, ids)
	if err != nil {
		return nil, err
	}
	stored := map[int64]bool{}
	for _, r := range results {
		stored[r.GameID] = true
	}
	games, err := s.store.ListGames(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return nil, err
	}
	for _, g := range games {
		if g.Aborted() || stored[g.ID] {
			continue
		}
		darts, err := s.store.ListDarts(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		state, err := g.Play(darts)
		if err != nil {
			return nil, err
		}
		if !g.Completed(darts, state.Common(), s.now()) {
			continue
		}
		added := stats.ResultsOf(g.ID, state)
		if err := s.store.AddResults(ctx, added); err != nil {
			return nil, err
		}
		results = append(results, added...)
	}
	for _, r := range results {
		for i := range summaries {
			if summaries[i].Player.ID == r.PlayerID {
				summaries[i].AddResult(r)
			}
		}
	}

	// Finished tournaments: stored like the games, once their last match is completed.
	tournamentResults, err := s.store.ListTournamentResults(ctx, ids)
	if err != nil {
		return nil, err
	}
	storedTournaments := map[int64]bool{}
	for _, r := range tournamentResults {
		storedTournaments[r.TournamentID] = true
	}
	tournaments, err := s.store.ListTournaments(ctx, acc.ID, acc.PlayerID, 0)
	if err != nil {
		return nil, err
	}
	for _, t := range tournaments {
		if t.Aborted() || storedTournaments[t.ID] {
			continue
		}
		v, err := s.tournamentView(ctx, acc, t)
		if err != nil {
			return nil, err
		}
		if !v.State.Over {
			continue
		}
		var added []stats.TournamentResult
		for _, id := range t.PlayerIDs() {
			added = append(added, stats.TournamentResult{TournamentID: t.ID, PlayerID: id, Mode: t.Settings.Mode, Won: *v.State.WinnerID == id})
		}
		if err := s.store.AddTournamentResults(ctx, added); err != nil {
			return nil, err
		}
		tournamentResults = append(tournamentResults, added...)
	}
	for _, r := range tournamentResults {
		for i := range summaries {
			if summaries[i].Player.ID == r.PlayerID {
				summaries[i].AddTournamentResult(r)
			}
		}
	}
	return summaries, nil
}
