package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/uptrace/bun"
)

var ErrUsernameTaken = errors.New("username already taken")

// CreateAccount stores the account together with its player.
func (store *Store) CreateAccount(ctx context.Context, a *account.Account, p *player.Player) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		exists, err := tx.NewSelect().Model((*account.Account)(nil)).Where("username = ?", a.Username).Exists(ctx)
		if err != nil {
			return err
		}
		if exists {
			return ErrUsernameTaken
		}

		if _, err := tx.NewInsert().Model(p).Exec(ctx); err != nil {
			return err
		}
		a.PlayerID = p.ID
		_, err = tx.NewInsert().Model(a).Returning("*").Exec(ctx)
		return err
	})
}

func (store *Store) GetAccount(ctx context.Context, id int64) (account.Account, error) {
	var a account.Account
	err := store.db.NewSelect().Model(&a).Where("id = ?", id).Scan(ctx)
	return a, notFound(err)
}

func (store *Store) GetAccountByUsername(ctx context.Context, username string) (account.Account, error) {
	var a account.Account
	err := store.db.NewSelect().Model(&a).Where("username = ?", username).Scan(ctx)
	return a, notFound(err)
}

func (store *Store) CreateSession(ctx context.Context, s *account.Session) error {
	_, err := store.db.NewInsert().Model(s).Exec(ctx)
	return err
}

// GetSession returns the session with the token hash, ErrNotFound if it doesn't exist or is expired.
func (store *Store) GetSession(ctx context.Context, tokenHash string) (account.Session, error) {
	var s account.Session
	err := store.db.NewSelect().Model(&s).Where("token_hash = ?", tokenHash).Scan(ctx)
	if err != nil {
		return s, notFound(err)
	}
	if time.Now().After(s.ExpiresAt) {
		_ = store.DeleteSession(ctx, tokenHash)
		return s, ErrNotFound
	}
	return s, nil
}

func (store *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := store.db.NewDelete().Model((*account.Session)(nil)).Where("token_hash = ?", tokenHash).Exec(ctx)
	return err
}

// notFound translates sql.ErrNoRows into ErrNotFound.
// SetLanguage stores the language of an account's web interface.
func (store *Store) SetLanguage(ctx context.Context, a *account.Account) error {
	_, err := store.db.NewUpdate().Model(a).Column("language").WherePK().Exec(ctx)
	return err
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
