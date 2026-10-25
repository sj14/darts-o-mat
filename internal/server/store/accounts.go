package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

var (
	ErrUsernameTaken = errors.New("username already taken")
	ErrEmailTaken    = errors.New("email already taken")
	// ErrLastAdmin means the last admin can't delete their account or give up the role: the
	// server would be left without one.
	ErrLastAdmin = errors.New("the last admin can't delete their account or give up the role")
)

// CreateAccount stores the account together with its player; the first account becomes the
// admin.
func (store *Store) CreateAccount(ctx context.Context, a *account.Account, p *player.Player) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		exists, err := tx.NewSelect().Model((*account.Account)(nil)).Where("username = ?", a.Username).Exists(ctx)
		if err != nil {
			return err
		}
		if exists {
			return ErrUsernameTaken
		}
		if taken, err := emailTaken(ctx, tx, a.Email, 0); err != nil {
			return err
		} else if taken {
			return ErrEmailTaken
		}

		// The first account is the admin.
		others, err := tx.NewSelect().Model((*account.Account)(nil)).Exists(ctx)
		if err != nil {
			return err
		}
		a.Admin = !others

		if _, err := tx.NewInsert().Model(p).Exec(ctx); err != nil {
			return err
		}
		a.PlayerID = p.ID
		_, err = tx.NewInsert().Model(a).Returning("*").Exec(ctx)
		return err
	})
}

// emailTaken reports whether an account other than the one with the ID has the email; an
// empty email is never taken.
func emailTaken(ctx context.Context, tx bun.Tx, email string, id int64) (bool, error) {
	if email == "" {
		return false, nil
	}
	return tx.NewSelect().Model((*account.Account)(nil)).Where("email = ?", email).Where("id <> ?", id).Exists(ctx)
}

// SetEmail stores the email of an account, ErrEmailTaken if another account has it.
func (store *Store) SetEmail(ctx context.Context, id int64, email string) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if taken, err := emailTaken(ctx, tx, email, id); err != nil {
			return err
		} else if taken {
			return ErrEmailTaken
		}
		_, err := tx.NewUpdate().Model((*account.Account)(nil)).Set("email = ?", email).Where("id = ?", id).Exec(ctx)
		return err
	})
}

// OtherAdmin says whether an account other than the given one is an admin.
func (store *Store) OtherAdmin(ctx context.Context, accountID int64) (bool, error) {
	return store.db.NewSelect().Model((*account.Account)(nil)).Where("admin AND id <> ?", accountID).Exists(ctx)
}

// ListAdmins returns the admins, by username.
func (store *Store) ListAdmins(ctx context.Context) ([]account.Account, error) {
	var admins []account.Account
	err := store.db.NewSelect().Model(&admins).Where("admin").Order("username").Scan(ctx)
	return admins, err
}

// SetAdmin gives an account the admin role or takes it away; ErrLastAdmin if it is the
// last admin.
func (store *Store) SetAdmin(ctx context.Context, id int64, admin bool) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if !admin {
			if err := keepAdmin(ctx, tx, id); err != nil {
				return err
			}
		}
		_, err := tx.NewUpdate().Model((*account.Account)(nil)).Set("admin = ?", admin).Where("id = ?", id).Exec(ctx)
		return err
	})
}

// keepAdmin returns ErrLastAdmin if the account is the only admin, before the transaction
// takes the role from it or deletes it. On PostgreSQL it locks the admins' rows (in the
// order of their IDs, so two transactions don't deadlock): a second transaction waits and
// then no longer counts the admin the first one took away, so two admins can't each take
// away one of the last two. SQLite has one connection, its transactions don't overlap.
func keepAdmin(ctx context.Context, tx bun.Tx, id int64) error {
	q := tx.NewSelect().Model((*account.Account)(nil)).Column("id").Where("admin").Order("id")
	if tx.Dialect().Name() == dialect.PG {
		q = q.For("UPDATE")
	}
	var admins []int64
	if err := q.Scan(ctx, &admins); err != nil {
		return err
	}
	if len(admins) == 1 && admins[0] == id {
		return ErrLastAdmin
	}
	return nil
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

// SetInvisible stores whether the account hides from its friends that it is online.
func (store *Store) SetInvisible(ctx context.Context, a *account.Account) error {
	_, err := store.db.NewUpdate().Model(a).Column("invisible").WherePK().Exec(ctx)
	return err
}

// SetLastSeen stores when the account was last online.
func (store *Store) SetLastSeen(ctx context.Context, accountID int64, t time.Time) error {
	_, err := store.db.NewUpdate().Model((*account.Account)(nil)).Set("last_seen_at = ?", t).Where("id = ?", accountID).Exec(ctx)
	return err
}
