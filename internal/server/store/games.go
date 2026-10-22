package store

import (
	"context"
	"errors"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/board"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/throw"
	"github.com/uptrace/bun"
)

var ErrNotFound = errors.New("not found")

// CreateGame stores the game with its participants (in the order given, each with the
// board it throws on) and marks their boards as used.
func (store *Store) CreateGame(ctx context.Context, g *game.Game) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if err := saveSettings(ctx, tx, &g.Settings); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(g).Returning("*").Exec(ctx); err != nil {
			return err
		}
		for i := range g.Participants {
			p := &g.Participants[i]
			p.GameID, p.Position = g.ID, i
			if _, err := tx.NewInsert().Model(p).Exec(ctx); err != nil {
				return err
			}
			if err := markBoardUsed(ctx, tx, p.BoardID, g.CreatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// markBoardUsed sets the time a board was last used, if there is one.
func markBoardUsed(ctx context.Context, tx bun.Tx, boardID *int64, at time.Time) error {
	if boardID == nil {
		return nil
	}
	_, err := tx.NewUpdate().Model((*board.Board)(nil)).Set("last_used_at = ?", at).Where("id = ?", *boardID).Exec(ctx)
	return err
}

// loadParticipants fills in the participants of games, in throwing order.
func (store *Store) loadParticipants(ctx context.Context, games []game.Game) error {
	if len(games) == 0 {
		return nil
	}
	index := make(map[int64]int, len(games))
	ids := make([]int64, len(games))
	for i, g := range games {
		index[g.ID], ids[i] = i, g.ID
	}
	var rows []game.Participant
	if err := store.db.NewSelect().Model(&rows).Where("game_id IN (?)", bun.In(ids)).OrderExpr("game_id, place").Scan(ctx); err != nil {
		return err
	}
	for _, r := range rows {
		games[index[r.GameID]].Participants = append(games[index[r.GameID]].Participants, r)
	}
	return nil
}

// scanGames runs the query and loads the participants and the mode settings of the games it found.
func (store *Store) scanGames(ctx context.Context, q *bun.SelectQuery) ([]game.Game, error) {
	games := []game.Game{}
	if err := q.Model(&games).Scan(ctx); err != nil {
		return nil, err
	}
	if err := store.loadParticipants(ctx, games); err != nil {
		return nil, err
	}
	settings := make([]*game.Settings, len(games))
	for i := range games {
		settings[i] = &games[i].Settings
	}
	return games, loadSettings(ctx, store.db, settings)
}

func (store *Store) GetGame(ctx context.Context, id int64) (game.Game, error) {
	games, err := store.scanGames(ctx, store.db.NewSelect().Where("id = ?", id))
	if err != nil {
		return game.Game{}, err
	}
	if len(games) == 0 {
		return game.Game{}, ErrNotFound
	}
	return games[0], nil
}

// ListGames returns the most recent games with a player of the account (its own player
// or one of its guests), newest first. A limit of 0 returns all of them.
func (store *Store) ListGames(ctx context.Context, accountID, playerID int64, limit int64) ([]game.Game, error) {
	mine := store.db.NewSelect().Model((*game.Participant)(nil)).ColumnExpr("participant.game_id").
		Join("JOIN players AS p ON p.id = participant.player_id").
		Where("p.id = ? OR p.guest_of = ?", playerID, accountID)
	q := store.db.NewSelect().Where("id IN (?)", mine).Order("id DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	return store.scanGames(ctx, q)
}

// ListUnfinishedGamesByBoard returns the games a player throws on the board in that may
// still be running, newest first: started (ready), not aborted and not completed. Whether
// one is over only its replay tells, so the caller still checks that; this only leaves
// out the games that can't be running, without replaying them.
func (store *Store) ListUnfinishedGamesByBoard(ctx context.Context, boardID int64) ([]game.Game, error) {
	on := store.db.NewSelect().Model((*game.Participant)(nil)).Column("game_id").Where("board_id = ?", boardID)
	return store.scanGames(ctx, store.db.NewSelect().Where("id IN (?)", on).
		Where("started_at IS NOT NULL").Where("aborted_at IS NULL").Where("completed_at IS NULL").
		Order("id DESC"))
}

// ListUnfinishedGamesByPlayer returns the games the player took a seat in (accepted) that
// may not be finished, newest first: in the lobby or running, neither aborted nor
// completed. Like ListUnfinishedGamesByBoard, the caller replays them to see which are over.
func (store *Store) ListUnfinishedGamesByPlayer(ctx context.Context, playerID int64) ([]game.Game, error) {
	seated := store.db.NewSelect().Model((*game.Participant)(nil)).Column("game_id").
		Where("player_id = ?", playerID).Where("status = ?", game.InvitationAccepted)
	return store.scanGames(ctx, store.db.NewSelect().Where("id IN (?)", seated).
		Where("aborted_at IS NULL").Where("completed_at IS NULL").
		Order("id DESC"))
}

// ListInvitedGames returns the games a player was invited to and hasn't answered yet,
// newest first.
func (store *Store) ListInvitedGames(ctx context.Context, playerID int64) ([]game.Game, error) {
	pending := store.db.NewSelect().Model((*game.Participant)(nil)).Column("game_id").
		Where("player_id = ?", playerID).Where("status = ?", game.InvitationPending)
	return store.scanGames(ctx, store.db.NewSelect().Where("id IN (?)", pending).Order("id DESC"))
}

// AcceptInvitation accepts the invitation of a player, who throws on the board if one is given.
func (store *Store) AcceptInvitation(ctx context.Context, gameID, playerID int64, boardID *int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewUpdate().Model((*game.Participant)(nil)).
			Set("status = ?", game.InvitationAccepted).
			Set("board_id = ?", boardID).
			Where("game_id = ?", gameID).
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

// SetParticipantBoard puts a player of a game on a board, or takes them off one with a
// nil board, and marks the board as used at that time.
func (store *Store) SetParticipantBoard(ctx context.Context, gameID, playerID int64, boardID *int64, at time.Time) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*game.Participant)(nil)).Set("board_id = ?", boardID).
			Where("game_id = ?", gameID).Where("player_id = ?", playerID).Exec(ctx); err != nil {
			return err
		}
		return markBoardUsed(ctx, tx, boardID, at)
	})
}

// ReleaseBoard takes a board from the players of a game.
func (store *Store) ReleaseBoard(ctx context.Context, gameID, boardID int64) error {
	_, err := store.db.NewUpdate().Model((*game.Participant)(nil)).Set("board_id = NULL").
		Where("game_id = ?", gameID).Where("board_id = ?", boardID).Exec(ctx)
	return err
}

// RemoveParticipant removes a player from a game that hasn't started (declined, withdrawn
// or left in the lobby). A game that nobody accepted is deleted with its invitations.
func (store *Store) RemoveParticipant(ctx context.Context, gameID, playerID int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.NewDelete().Model((*game.Participant)(nil)).
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
		// Invitations to a game that nobody accepted (anymore) are worth nothing.
		accepted, err := tx.NewSelect().Model((*game.Participant)(nil)).Where("game_id = ?", gameID).Where("status = ?", game.InvitationAccepted).Count(ctx)
		if err != nil || accepted > 0 {
			return err
		}
		if _, err := tx.NewDelete().Model((*game.Participant)(nil)).Where("game_id = ?", gameID).Exec(ctx); err != nil {
			return err
		}
		var g game.Game
		if err := tx.NewSelect().Model(&g).Column("mode", "settings_id").Where("id = ?", gameID).Scan(ctx); err != nil {
			return err
		}
		if _, err = tx.NewDelete().Model((*game.Game)(nil)).Where("id = ?", gameID).Exec(ctx); err != nil {
			return err
		}
		return deleteSettings(ctx, tx, g.Settings)
	})
}

// AbortGame marks a game as aborted.
func (store *Store) AbortGame(ctx context.Context, g *game.Game) error {
	_, err := store.db.NewUpdate().Model(g).Column("aborted_at").WherePK().Exec(ctx)
	return err
}

// CompleteGame stores when a game was completed, unless that is stored already.
func (store *Store) CompleteGame(ctx context.Context, g *game.Game) error {
	_, err := store.db.NewUpdate().Model(g).Column("completed_at").WherePK().Where("completed_at IS NULL").Exec(ctx)
	return err
}

// ListGamesWithDartsSince returns the games of the account (a player of it, or a guest)
// that aren't completed or aborted and had a dart since the time; those are the ones that
// can have just finished.
func (store *Store) ListGamesWithDartsSince(ctx context.Context, accountID, playerID int64, since time.Time) ([]game.Game, error) {
	mine := store.db.NewSelect().Model((*game.Participant)(nil)).ColumnExpr("participant.game_id").
		Join("JOIN players AS p ON p.id = participant.player_id").
		Where("p.id = ? OR p.guest_of = ?", playerID, accountID)
	recent := store.db.NewSelect().Model((*throw.Dart)(nil)).Column("game_id").Where("created_at > ?", since)
	return store.scanGames(ctx, store.db.NewSelect().
		Where("completed_at IS NULL").
		Where("aborted_at IS NULL").
		Where("id IN (?)", mine).
		Where("id IN (?)", recent))
}

// StartGame stores when a game became ready.
func (store *Store) StartGame(ctx context.Context, g *game.Game) error {
	_, err := store.db.NewUpdate().Model(g).Column("started_at").WherePK().Exec(ctx)
	return err
}

// SetBullOffCloser stores who the players found closer when bulling off ended in a tie.
func (store *Store) SetBullOffCloser(ctx context.Context, g *game.Game) error {
	_, err := store.db.NewUpdate().Model(g).Column("bull_off_closer").WherePK().Exec(ctx)
	return err
}

func (store *Store) AddDart(ctx context.Context, d *throw.Dart) error {
	_, err := store.db.NewInsert().Model(d).Exec(ctx)
	return err
}

// ListDarts returns a game's log, darts and takeouts, in the order they happened.
func (store *Store) ListDarts(ctx context.Context, gameID int64) ([]throw.Dart, error) {
	darts := []throw.Dart{}
	err := store.db.NewSelect().Model(&darts).Where("game_id = ?", gameID).Order("id").Scan(ctx)
	return darts, err
}

// DeleteLastDart removes the most recent entry of a game's log (a dart or a takeout),
// ErrNotFound if there is none.
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

// UpdateDart changes what a dart hit.
func (store *Store) UpdateDart(ctx context.Context, d *throw.Dart) error {
	res, err := store.db.NewUpdate().Model(d).Column("number", "segment").WherePK().Where("game_id = ?", d.GameID).Exec(ctx)
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

// JoinGame adds a player to a game with an accepted invitation, as the last to throw,
// on the board if one is given.
func (store *Store) JoinGame(ctx context.Context, gameID, playerID int64, boardID *int64) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var last int
		if err := tx.NewSelect().Model((*game.Participant)(nil)).ColumnExpr("COALESCE(MAX(place), -1)").Where("game_id = ?", gameID).Scan(ctx, &last); err != nil {
			return err
		}
		p := game.Participant{GameID: gameID, PlayerID: playerID, Position: last + 1, Status: game.InvitationAccepted, BoardID: boardID}
		if _, err := tx.NewInsert().Model(&p).Exec(ctx); err != nil {
			return err
		}
		return markBoardUsed(ctx, tx, boardID, time.Now())
	})
}

// ListOpenGames returns the games open for strangers that haven't started, newest first.
func (store *Store) ListOpenGames(ctx context.Context) ([]game.Game, error) {
	return store.scanGames(ctx, store.db.NewSelect().
		Where("seats > 0").
		Where("started_at IS NULL").
		Where("aborted_at IS NULL").
		Order("id DESC"))
}

// ListTimedGames returns the games with a time limit for turns that aren't aborted and
// started or had a dart since the time.
func (store *Store) ListTimedGames(ctx context.Context, since time.Time) ([]game.Game, error) {
	recent := store.db.NewSelect().Model((*throw.Dart)(nil)).Column("game_id").Where("created_at > ?", since)
	return store.scanGames(ctx, store.db.NewSelect().
		Where("turn_seconds > 0").
		Where("aborted_at IS NULL").
		Where("started_at > ? OR id IN (?)", since, recent))
}
