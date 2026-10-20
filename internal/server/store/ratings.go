package store

import (
	"context"

	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
)

// SetRating stores the thumb a player gave another in a game, replacing an earlier one.
func (store *Store) SetRating(ctx context.Context, r player.Rating) error {
	_, err := store.db.NewInsert().Model(&r).
		On("CONFLICT (game_id, rater_id, rated_id) DO UPDATE").Set("up = EXCLUDED.up").Exec(ctx)
	return err
}

// ListRatingsBy returns the thumbs the player gave in a game.
func (store *Store) ListRatingsBy(ctx context.Context, gameID, raterID int64) ([]player.Rating, error) {
	ratings := []player.Rating{}
	err := store.db.NewSelect().Model(&ratings).Where("game_id = ?", gameID).Where("rater_id = ?", raterID).Scan(ctx)
	return ratings, err
}

// Reputations returns the thumbs the players got, by player; players without any aren't in it.
// Every player counts once for each other: only the thumb they gave in the newest game (the
// highest ID) counts, so playing many games against one friend doesn't add up.
func (store *Store) Reputations(ctx context.Context, playerIDs []int64) (map[int64]player.Reputation, error) {
	reputations := map[int64]player.Reputation{}
	if len(playerIDs) == 0 {
		return reputations, nil
	}
	var rows []struct {
		RatedID int64
		Up      bool
		N       int
	}
	err := store.db.NewSelect().Model((*player.Rating)(nil)).ColumnExpr("rated_id, up, COUNT(*) AS n").
		Where("rated_id IN (?)", bun.In(playerIDs)).
		Where("game_id = (SELECT MAX(r2.game_id) FROM ratings AS r2 WHERE r2.rater_id = rating.rater_id AND r2.rated_id = rating.rated_id)").
		GroupExpr("rated_id, up").Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		r := reputations[row.RatedID]
		if row.Up {
			r.Up = row.N
		} else {
			r.Down = row.N
		}
		reputations[row.RatedID] = r
	}
	return reputations, nil
}
