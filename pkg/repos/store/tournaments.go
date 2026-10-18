package store

import (
	"context"
	"math/rand"
	"slices"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/tournament"
	"github.com/uptrace/bun"
)

// CreateTournament stores the tournament and a pending invitation for each of the
// invited players.
func (store *Store) CreateTournament(ctx context.Context, t *tournament.Tournament, invitedPlayerIDs []int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(t).Returning("*").Exec(ctx); err != nil {
			return err
		}
		for _, id := range invitedPlayerIDs {
			inv := tournament.Invitation{TournamentID: t.ID, PlayerID: id, Status: game.InvitationPending}
			if _, err := tx.NewInsert().Model(&inv).Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (store *Store) GetTournament(ctx context.Context, id int64) (tournament.Tournament, error) {
	var t tournament.Tournament
	err := store.db.NewSelect().Model(&t).Where("id = ?", id).Scan(ctx)
	return t, notFound(err)
}

// ListTournaments returns the tournaments an account created or was invited to, newest
// first. A limit of 0 returns all of them.
func (store *Store) ListTournaments(ctx context.Context, accountID, playerID int64, limit int64) ([]tournament.Tournament, error) {
	invited := store.db.NewSelect().Model((*tournament.Invitation)(nil)).Column("tournament_id").Where("player_id = ?", playerID)
	ts := []tournament.Tournament{}
	q := store.db.NewSelect().Model(&ts).
		Where("owner_account_id = ?", accountID).
		WhereOr("id IN (?)", invited).
		Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return ts, q.Scan(ctx)
}

func (store *Store) ListTournamentInvitations(ctx context.Context, tournamentID int64) ([]tournament.Invitation, error) {
	invitations := []tournament.Invitation{}
	err := store.db.NewSelect().Model(&invitations).Where("tournament_id = ?", tournamentID).Scan(ctx)
	return invitations, err
}

// ListPendingTournamentInvitations returns the tournament invitations a player has not
// answered yet, newest first.
func (store *Store) ListPendingTournamentInvitations(ctx context.Context, playerID int64) ([]tournament.Invitation, error) {
	invitations := []tournament.Invitation{}
	err := store.db.NewSelect().Model(&invitations).
		Where("player_id = ?", playerID).
		Where("status = ?", game.InvitationPending).
		Order("tournament_id DESC").
		Scan(ctx)
	return invitations, err
}

// AcceptTournamentInvitation accepts the invitation of a player, who throws on the board
// if one is given.
func (store *Store) AcceptTournamentInvitation(ctx context.Context, tournamentID, playerID int64, boardID *int64) error {
	res, err := store.db.NewUpdate().Model((*tournament.Invitation)(nil)).
		Set("status = ?", game.InvitationAccepted).
		Set("board_id = ?", boardID).
		Where("tournament_id = ?", tournamentID).
		Where("player_id = ?", playerID).
		Where("status = ?", game.InvitationPending).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveTournamentPlayer deletes the invitation of a player and removes the player
// from the tournament.
func (store *Store) RemoveTournamentPlayer(ctx context.Context, tournamentID, playerID int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().Model((*tournament.Invitation)(nil)).
			Where("tournament_id = ?", tournamentID).
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
		var t tournament.Tournament
		if err := tx.NewSelect().Model(&t).Where("id = ?", tournamentID).Scan(ctx); err != nil {
			return notFound(err)
		}
		t.PlayerIDs = slices.DeleteFunc(t.PlayerIDs, func(id int64) bool { return id == playerID })
		_, err = tx.NewUpdate().Model(&t).Column("player_ids").WherePK().Exec(ctx)
		return err
	})
}

// AbortTournament marks a tournament as aborted.
func (store *Store) AbortTournament(ctx context.Context, t *tournament.Tournament) error {
	_, err := store.db.NewUpdate().Model(t).Column("aborted_at").WherePK().Exec(ctx)
	return err
}

// ListTournamentGames returns the games of a tournament, oldest first.
func (store *Store) ListTournamentGames(ctx context.Context, tournamentID int64) ([]game.Game, error) {
	games := []game.Game{}
	err := store.db.NewSelect().Model(&games).Where("tournament_id = ?", tournamentID).Order("id").Scan(ctx)
	return games, err
}

// JoinTournament adds a player to a tournament with an accepted invitation. The player
// that takes the last seat completes the draw: the players are shuffled, as when a
// tournament that isn't open is created.
func (store *Store) JoinTournament(ctx context.Context, tournamentID, playerID int64, boardID *int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var t tournament.Tournament
		if err := tx.NewSelect().Model(&t).Where("id = ?", tournamentID).Scan(ctx); err != nil {
			return notFound(err)
		}
		t.PlayerIDs = append(t.PlayerIDs, playerID)
		if t.Full(len(t.PlayerIDs)) {
			rand.Shuffle(len(t.PlayerIDs), func(i, j int) { t.PlayerIDs[i], t.PlayerIDs[j] = t.PlayerIDs[j], t.PlayerIDs[i] })
		}
		if _, err := tx.NewUpdate().Model(&t).Column("player_ids").WherePK().Exec(ctx); err != nil {
			return err
		}
		inv := tournament.Invitation{TournamentID: tournamentID, PlayerID: playerID, Status: game.InvitationAccepted, BoardID: boardID}
		_, err := tx.NewInsert().Model(&inv).Exec(ctx)
		return err
	})
}

// ListOpenTournaments returns the tournaments open for strangers that were created
// since the time, newest first, including full and aborted ones, which the caller sorts out.
func (store *Store) ListOpenTournaments(ctx context.Context, since time.Time) ([]tournament.Tournament, error) {
	ts := []tournament.Tournament{}
	err := store.db.NewSelect().Model(&ts).
		Where(`"open" = ?`, true).
		Where("created_at > ?", since).
		Where("aborted_at IS NULL").
		Order("id DESC").
		Scan(ctx)
	return ts, err
}
