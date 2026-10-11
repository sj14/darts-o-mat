package store

import (
	"context"
	"errors"
	"slices"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/uptrace/bun"
)

var ErrNotFound = errors.New("not found")

// CreateGame stores the game and a pending invitation for each of the invited players.
func (store *Store) CreateGame(ctx context.Context, g *game.Game, invitedPlayerIDs []int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(g).Returning("*").Exec(ctx); err != nil {
			return err
		}
		for _, id := range invitedPlayerIDs {
			inv := game.Invitation{GameID: g.ID, PlayerID: id, Status: game.InvitationPending}
			if _, err := tx.NewInsert().Model(&inv).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (store *Store) GetGame(ctx context.Context, id int64) (game.Game, error) {
	var g game.Game
	err := store.db.NewSelect().Model(&g).Where("id = ?", id).Scan(ctx)
	return g, notFound(err)
}

// ListGames returns the most recent games an account created or was invited to, newest first.
func (store *Store) ListGames(ctx context.Context, accountID, playerID int64, limit int) ([]game.Game, error) {
	invited := store.db.NewSelect().Model((*game.Invitation)(nil)).Column("game_id").Where("player_id = ?", playerID)

	games := []game.Game{}
	err := store.db.NewSelect().Model(&games).
		Where("owner_account_id = ?", accountID).
		WhereOr("id IN (?)", invited).
		Order("id DESC").
		Limit(limit).
		Scan(ctx)
	return games, err
}

func (store *Store) ListInvitations(ctx context.Context, gameID int64) ([]game.Invitation, error) {
	invitations := []game.Invitation{}
	err := store.db.NewSelect().Model(&invitations).Where("game_id = ?", gameID).Scan(ctx)
	return invitations, err
}

// ListPendingInvitations returns the invitations a player has not answered yet, newest first.
func (store *Store) ListPendingInvitations(ctx context.Context, playerID int64) ([]game.Invitation, error) {
	invitations := []game.Invitation{}
	err := store.db.NewSelect().Model(&invitations).
		Where("player_id = ?", playerID).
		Where("status = ?", game.InvitationPending).
		Order("game_id DESC").
		Scan(ctx)
	return invitations, err
}

func (store *Store) AcceptInvitation(ctx context.Context, gameID, playerID int64) error {
	res, err := store.db.NewUpdate().Model((*game.Invitation)(nil)).
		Set("status = ?", game.InvitationAccepted).
		Where("game_id = ?", gameID).
		Where("player_id = ?", playerID).
		Where("status = ?", game.InvitationPending).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveInvitedPlayer deletes the invitation of a player and removes the player from the game.
// A game without players left is deleted.
func (store *Store) RemoveInvitedPlayer(ctx context.Context, gameID, playerID int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().Model((*game.Invitation)(nil)).
			Where("game_id = ?", gameID).
			Where("player_id = ?", playerID).
			Exec(ctx)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return ErrNotFound
		}

		var g game.Game
		if err := tx.NewSelect().Model(&g).Where("id = ?", gameID).Scan(ctx); err != nil {
			return notFound(err)
		}
		g.PlayerIDs = slices.DeleteFunc(g.PlayerIDs, func(id int64) bool { return id == playerID })

		if len(g.PlayerIDs) == 0 {
			_, err = tx.NewDelete().Model(&g).WherePK().Exec(ctx)
			return err
		}
		_, err = tx.NewUpdate().Model(&g).Column("player_ids").WherePK().Exec(ctx)
		return err
	})
}

func (store *Store) AddDart(ctx context.Context, d *throw.Dart) error {
	_, err := store.db.NewInsert().Model(d).Exec(ctx)
	return err
}

// ListDarts returns the darts of a game in the order they were thrown.
func (store *Store) ListDarts(ctx context.Context, gameID int64) ([]throw.Dart, error) {
	darts := []throw.Dart{}
	err := store.db.NewSelect().Model(&darts).Where("game_id = ?", gameID).Order("id").Scan(ctx)
	return darts, err
}

// DeleteLastDart removes the most recent dart of a game, ErrNotFound if there is none.
func (store *Store) DeleteLastDart(ctx context.Context, gameID int64) error {
	last := store.db.NewSelect().Model((*throw.Dart)(nil)).ColumnExpr("MAX(id)").Where("game_id = ?", gameID)
	res, err := store.db.NewDelete().Model((*throw.Dart)(nil)).Where("id = (?)", last).Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
