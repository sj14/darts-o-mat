package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
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

// ListRecentResults returns the stored results of the players in games of the mode, the
// most recent limit of them per player.
func (store *Store) ListRecentResults(ctx context.Context, playerIDs []int64, mode game.Mode, limit int) ([]stats.Result, error) {
	results := []stats.Result{}
	if len(playerIDs) == 0 {
		return results, nil
	}
	ranked := store.db.NewSelect().Model((*stats.Result)(nil)).ColumnExpr("*").
		ColumnExpr("ROW_NUMBER() OVER (PARTITION BY player_id ORDER BY game_id DESC) AS n").
		Where("player_id IN (?)", bun.In(playerIDs)).Where("mode = ?", mode)
	err := store.db.NewSelect().Model(&results).ModelTableExpr("(?) AS result", ranked).Where("n <= ?", limit).Scan(ctx)
	return results, err
}

// ListPositions returns where the cameras saw the player's most recent darts, at most
// limit of them; with a mode only those of games of that mode.
func (store *Store) ListPositions(ctx context.Context, playerID int64, mode game.Mode, limit int64) ([]dartboard.Point, error) {
	var darts []throw.Dart
	q := store.db.NewSelect().Model(&darts).Column("x", "y").
		Where("player_id = ?", playerID).Where("x IS NOT NULL").Where("y IS NOT NULL")
	if mode != "" {
		q = q.Where("game_id IN (SELECT id FROM games WHERE mode = ?)", mode)
	}
	err := q.Order("id DESC").Limit(limit).Scan(ctx)
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

// AddQuits stores the games or tournaments players left; one that is stored already stays.
func (store *Store) AddQuits(ctx context.Context, quits []stats.Quit) error {
	if len(quits) == 0 {
		return nil
	}
	_, err := store.db.NewInsert().Model(&quits).On("CONFLICT (player_id, tournament, target_id) DO NOTHING").Exec(ctx)
	return err
}

// ListQuits returns the games and tournaments the players left.
func (store *Store) ListQuits(ctx context.Context, playerIDs []int64) ([]stats.Quit, error) {
	quits := []stats.Quit{}
	if len(playerIDs) == 0 {
		return quits, nil
	}
	err := store.db.NewSelect().Model(&quits).Where("player_id IN (?)", bun.In(playerIDs)).Scan(ctx)
	return quits, err
}
