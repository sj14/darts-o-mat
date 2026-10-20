package service

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
)

// HeatDarts is how many of a player's most recent camera darts the heatmap shows (of one
// mode, if the heatmap is narrowed down to it).
const HeatDarts = 5000

// Stats returns the statistics of the account's player and its guests over all completed
// games and finished tournaments, the ones they left themselves (stats.Quit), and where their darts hit the board (heatMode narrows
// the latter down to the games of one mode, empty means all).
//
// Once a game is completed nothing about it changes anymore, so its result per player
// is stored (stats.Result) the first time it is seen here and added up from then on;
// only games that aren't stored yet are replayed. Games that are over but not completed
// (a dart could still be corrected) don't count yet.
func (s *Service) Stats(ctx context.Context, acc account.Account, heatMode game.Mode) ([]stats.Summary, error) {
	players, err := s.store.ListPlayers(ctx, acc.ID, acc.PlayerID)
	if err != nil {
		return nil, err
	}
	summaries := make([]stats.Summary, len(players))
	ids := make([]int64, len(players))
	for i, p := range players {
		summaries[i].Player = p
		ids[i] = p.ID
		points, err := s.store.ListPositions(ctx, p.ID, heatMode, HeatDarts)
		if err != nil {
			return nil, err
		}
		summaries[i].Heat = stats.NewHeat(points)
	}

	reputations, err := s.store.Reputations(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range summaries {
		summaries[i].Reputation = reputations[summaries[i].Player.ID]
	}
	quits, err := s.store.ListQuits(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, q := range quits {
		for i := range summaries {
			if summaries[i].Player.ID == q.PlayerID {
				summaries[i].AddQuit(q)
			}
		}
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
