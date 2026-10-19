package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/stats"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/uptrace/bun"
)

// AddResults stores the results of a completed game. A result that is stored already stays
// as it is, so storing a game twice (two requests at once) is harmless.
func (store *Store) AddResults(ctx context.Context, results []stats.Result) error {
	if len(results) == 0 {
		return nil
	}
	_, err := store.db.NewInsert().Model(&results).On("CONFLICT (game_id, player_id) DO NOTHING").Exec(ctx)
	return err
}

// ListResults returns the stored results of the players.
func (store *Store) ListResults(ctx context.Context, playerIDs []int64) ([]stats.Result, error) {
	results := []stats.Result{}
	if len(playerIDs) == 0 {
		return results, nil
	}
	err := store.db.NewSelect().Model(&results).Where("player_id IN (?)", bun.In(playerIDs)).Order("game_id").Scan(ctx)
	return results, err
}

// ListPositions returns where the cameras saw the player's most recent darts, at most
// limit of them.
func (store *Store) ListPositions(ctx context.Context, playerID int64, limit int64) ([]dartboard.Point, error) {
	var darts []throw.Dart
	err := store.db.NewSelect().Model(&darts).Column("x", "y").
		Where("player_id = ?", playerID).Where("x IS NOT NULL").Where("y IS NOT NULL").
		Order("id DESC").Limit(limit).Scan(ctx)
	points := make([]dartboard.Point, len(darts))
	for i, d := range darts {
		points[i] = dartboard.Point{X: *d.X, Y: *d.Y}
	}
	return points, err
}

// AddTournamentResults stores the results of a tournament that is over; stored ones stay.
func (store *Store) AddTournamentResults(ctx context.Context, results []stats.TournamentResult) error {
	if len(results) == 0 {
		return nil
	}
	_, err := store.db.NewInsert().Model(&results).On("CONFLICT (tournament_id, player_id) DO NOTHING").Exec(ctx)
	return err
}

// ListTournamentResults returns the stored tournament results of the players.
func (store *Store) ListTournamentResults(ctx context.Context, playerIDs []int64) ([]stats.TournamentResult, error) {
	results := []stats.TournamentResult{}
	if len(playerIDs) == 0 {
		return results, nil
	}
	err := store.db.NewSelect().Model(&results).Where("player_id IN (?)", bun.In(playerIDs)).Scan(ctx)
	return results, err
}
