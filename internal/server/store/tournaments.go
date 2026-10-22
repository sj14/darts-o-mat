package store

import (
	"context"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/tournament"
	"github.com/uptrace/bun"
)

// CreateTournament stores the tournament with its participants, in the order given.
func (store *Store) CreateTournament(ctx context.Context, t *tournament.Tournament) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := saveSettings(ctx, tx, &t.Settings); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(t).Returning("*").Exec(ctx); err != nil {
			return err
		}
		for i := range t.Participants {
			p := &t.Participants[i]
			p.TournamentID, p.Position = t.ID, i
			if _, err := tx.NewInsert().Model(p).Exec(ctx); err != nil {
				return err
			}
			if err := markBoardUsed(ctx, tx, p.BoardID, t.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// scanTournaments runs the query and loads the participants and the mode settings of the tournaments it found.
func (store *Store) scanTournaments(ctx context.Context, q *bun.SelectQuery) ([]tournament.Tournament, error) {
	ts := []tournament.Tournament{}
	if err := q.Model(&ts).Scan(ctx); err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return ts, nil
	}
	index := make(map[int64]int, len(ts))
	ids := make([]int64, len(ts))
	for i, t := range ts {
		index[t.ID], ids[i] = i, t.ID
	}
	var rows []tournament.Participant
	if err := store.db.NewSelect().Model(&rows).Where("tournament_id IN (?)", bun.In(ids)).OrderExpr("tournament_id, place").Scan(ctx); err != nil {
		return nil, err
	}
	for _, r := range rows {
		ts[index[r.TournamentID]].Participants = append(ts[index[r.TournamentID]].Participants, r)
	}
	settings := make([]*game.Settings, len(ts))
	for i := range ts {
		settings[i] = &ts[i].Settings
	}
	return ts, loadSettings(ctx, store.db, settings)
}

func (store *Store) GetTournament(ctx context.Context, id int64) (tournament.Tournament, error) {
	ts, err := store.scanTournaments(ctx, store.db.NewSelect().Where("id = ?", id))
	if err != nil {
		return tournament.Tournament{}, err
	}
	if len(ts) == 0 {
		return tournament.Tournament{}, ErrNotFound
	}
	return ts[0], nil
}

// ListTournaments returns the tournaments with a player of the account (its own player
// or one of its guests), newest first. A limit of 0 returns all of them.
func (store *Store) ListTournaments(ctx context.Context, accountID, playerID int64, limit int64) ([]tournament.Tournament, error) {
	mine := store.db.NewSelect().Model((*tournament.Participant)(nil)).ColumnExpr("participant.tournament_id").
		Join("JOIN players AS p ON p.id = participant.player_id").
		Where("p.id = ? OR p.guest_of = ?", playerID, accountID)
	q := store.db.NewSelect().Where("id IN (?)", mine).Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return store.scanTournaments(ctx, q)
}

// ListUnfinishedTournamentsByPlayer returns the tournaments the player took a seat in
// (accepted) that are neither aborted nor completed, newest first. Whether one is over its
// games tell, so the caller still checks that.
func (store *Store) ListUnfinishedTournamentsByPlayer(ctx context.Context, playerID int64) ([]tournament.Tournament, error) {
	seated := store.db.NewSelect().Model((*tournament.Participant)(nil)).Column("tournament_id").
		Where("player_id = ?", playerID).Where("status = ?", game.InvitationAccepted)
	return store.scanTournaments(ctx, store.db.NewSelect().Where("id IN (?)", seated).
		Where("aborted_at IS NULL").Where("completed_at IS NULL").Order("id DESC"))
}

// ListInvitedTournaments returns the tournaments a player was invited to and hasn't
// answered yet, newest first.
func (store *Store) ListInvitedTournaments(ctx context.Context, playerID int64) ([]tournament.Tournament, error) {
	pending := store.db.NewSelect().Model((*tournament.Participant)(nil)).Column("tournament_id").
		Where("player_id = ?", playerID).Where("status = ?", game.InvitationPending)
	return store.scanTournaments(ctx, store.db.NewSelect().Where("id IN (?)", pending).Order("id DESC"))
}

// AcceptTournamentInvitation accepts the invitation of a player, who throws on the board
// if one is given.
func (store *Store) AcceptTournamentInvitation(ctx context.Context, tournamentID, playerID int64, boardID *int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*tournament.Participant)(nil)).
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
		return markBoardUsed(ctx, tx, boardID, time.Now())
	})
}

// RemoveTournamentParticipant removes a player from a tournament that hasn't started
// (declined, withdrawn or left in the lobby). A tournament that nobody accepted is deleted
// with its invitations, like a game (RemoveParticipant).
func (store *Store) RemoveTournamentParticipant(ctx context.Context, tournamentID, playerID int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().Model((*tournament.Participant)(nil)).
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
		accepted, err := tx.NewSelect().Model((*tournament.Participant)(nil)).Where("tournament_id = ?", tournamentID).Where("status = ?", game.InvitationAccepted).Count(ctx)
		if err != nil || accepted > 0 {
			return err
		}
		if _, err := tx.NewDelete().Model((*tournament.Participant)(nil)).Where("tournament_id = ?", tournamentID).Exec(ctx); err != nil {
			return err
		}
		var t tournament.Tournament
		if err := tx.NewSelect().Model(&t).Column("mode", "settings_id").Where("id = ?", tournamentID).Scan(ctx); err != nil {
			return err
		}
		if _, err = tx.NewDelete().Model((*tournament.Tournament)(nil)).Where("id = ?", tournamentID).Exec(ctx); err != nil {
			return err
		}
		return deleteSettings(ctx, tx, t.Settings)
	})
}

// CompleteTournament stores when a tournament was completed, unless that is stored already.
func (store *Store) CompleteTournament(ctx context.Context, t *tournament.Tournament) error {
	_, err := store.db.NewUpdate().Model(t).Column("completed_at").WherePK().Where("completed_at IS NULL").Exec(ctx)
	return err
}

// AbortTournament marks a tournament as aborted.
func (store *Store) AbortTournament(ctx context.Context, t *tournament.Tournament) error {
	_, err := store.db.NewUpdate().Model(t).Column("aborted_at").WherePK().Exec(ctx)
	return err
}

// ListTournamentGames returns the games of a tournament, oldest first.
func (store *Store) ListTournamentGames(ctx context.Context, tournamentID int64) ([]game.Game, error) {
	return store.scanGames(ctx, store.db.NewSelect().Where("tournament_id = ?", tournamentID).Order("id"))
}

// AddTournamentParticipant adds a player to a tournament in the lobby, invited or accepted
// (p.Status), as the last so far, on the board if one is given.
func (store *Store) AddTournamentParticipant(ctx context.Context, tournamentID int64, p tournament.Participant) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var last int
		if err := tx.NewSelect().Model((*tournament.Participant)(nil)).ColumnExpr("COALESCE(MAX(place), -1)").Where("tournament_id = ?", tournamentID).Scan(ctx, &last); err != nil {
			return err
		}
		p.TournamentID, p.Position = tournamentID, last+1
		if _, err := tx.NewInsert().Model(&p).Exec(ctx); err != nil {
			return err
		}
		return markBoardUsed(ctx, tx, p.BoardID, time.Now())
	})
}

// StartTournament stores when the tournament started and the order of its players as
// drawn (t.Participants), the seeds of the bracket.
func (store *Store) StartTournament(ctx context.Context, t *tournament.Tournament) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model(t).Column("started_at").WherePK().Exec(ctx); err != nil {
			return err
		}
		for i := range t.Participants {
			t.Participants[i].Position = i
			if _, err := tx.NewUpdate().Model(&t.Participants[i]).Column("place").WherePK().Exec(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetTournamentParticipantBoard puts a player of a tournament on a board, or takes them off
// one with a nil board, like SetParticipantBoard.
func (store *Store) SetTournamentParticipantBoard(ctx context.Context, tournamentID, playerID int64, boardID *int64, at time.Time) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*tournament.Participant)(nil)).Set("board_id = ?", boardID).
			Where("tournament_id = ?", tournamentID).Where("player_id = ?", playerID).Exec(ctx); err != nil {
			return err
		}
		return markBoardUsed(ctx, tx, boardID, at)
	})
}

// ListOpenTournaments returns the tournaments open for strangers with a free seat, newest
// first.
func (store *Store) ListOpenTournaments(ctx context.Context) ([]tournament.Tournament, error) {
	taken := store.db.NewSelect().Model((*tournament.Participant)(nil)).ColumnExpr("count(*)").
		Where("participant.tournament_id = tournament.id")
	return store.scanTournaments(ctx, store.db.NewSelect().
		Where("seats > 0").
		Where("seats > (?)", taken).
		Where("aborted_at IS NULL").
		Order("id DESC"))
}
