package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/player"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
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
			return account.ErrUsernameTaken
		}
		if taken, err := emailTaken(ctx, tx, a.Email, 0); err != nil {
			return err
		} else if taken {
			return account.ErrEmailTaken
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

// SetEmail stores the email of an account, account.ErrEmailTaken if another account has it.
func (store *Store) SetEmail(ctx context.Context, id int64, email string) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if taken, err := emailTaken(ctx, tx, email, id); err != nil {
			return err
		} else if taken {
			return account.ErrEmailTaken
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

// ListAccounts returns up to limit accounts, the newest first; with a search only those
// whose username or email contains it.
func (store *Store) ListAccounts(ctx context.Context, search string, limit int64) ([]account.Account, error) {
	var accounts []account.Account
	q := store.db.NewSelect().Model(&accounts).Order("created_at DESC", "id DESC").Limit(limit)
	if search != "" {
		pattern := "%" + escapeLike(search) + "%"
		q = q.Where(`username LIKE ? ESCAPE '\' OR email LIKE ? ESCAPE '\'`, pattern, pattern)
	}
	err := q.Scan(ctx)
	return accounts, err
}

// escapeLike makes the wildcards of LIKE (% and _, both allowed in usernames and emails)
// match only themselves.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// SetDisabled disables an account, which ends its sessions, or enables it again;
// account.ErrAdminAccount for disabling an admin, checked in the update, so a role given
// meanwhile counts.
func (store *Store) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		q := tx.NewUpdate().Model((*account.Account)(nil)).Set("disabled = ?", disabled).Where("id = ?", id)
		if disabled {
			q = q.Where("NOT admin")
		}
		res, err := q.Exec(ctx)
		if err != nil {
			return err
		}
		if n, err := res.RowsAffected(); err != nil {
			return err
		} else if n == 0 {
			return account.ErrAdminAccount
		}
		if !disabled {
			return nil
		}
		_, err = tx.NewDelete().Model((*account.Session)(nil)).Where("account_id = ?", id).Exec(ctx)
		return err
	})
}

// SetAdmin gives an account the admin role or takes it away; account.ErrLastAdmin if it is the
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

// keepAdmin returns account.ErrLastAdmin if the account is the only admin, before the
// transaction takes the role from it or deletes it. On PostgreSQL it locks the admins' rows
// (in the order of their IDs, so two transactions don't deadlock): a second transaction
// waits and then no longer counts the admin the first one took away, so two admins can't
// each take away one of the last two. SQLite has one connection, its transactions don't
// overlap.
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
		return account.ErrLastAdmin
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

// CreateSession stores a new session and deletes the sessions of all accounts that expired
// by now: their cookies are gone with them, so no request would ever delete them.
func (store *Store) CreateSession(ctx context.Context, s *account.Session, now time.Time) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Model((*account.Session)(nil)).Where("expires_at <= ?", now).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewInsert().Model(s).Exec(ctx)
		return err
	})
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

// SetPassword stores the password hash of an account and ends its sessions except the one
// with keepTokenHash.
func (store *Store) SetPassword(ctx context.Context, id int64, hash, keepTokenHash string) error {
	return store.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model((*account.Account)(nil)).Set("password_hash = ?", hash).Where("id = ?", id).Exec(ctx); err != nil {
			return err
		}
		_, err := tx.NewDelete().Model((*account.Session)(nil)).Where("account_id = ? AND token_hash <> ?", id, keepTokenHash).Exec(ctx)
		return err
	})
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

// LastSeen returns when the accounts were last online, by account ID; an account that
// never was is missing.
func (store *Store) LastSeen(ctx context.Context, accountIDs []int64) (map[int64]time.Time, error) {
	seen := map[int64]time.Time{}
	if len(accountIDs) == 0 {
		return seen, nil
	}
	var accounts []account.Account
	err := store.db.NewSelect().Model(&accounts).Column("id", "last_seen_at").
		Where("id IN (?)", bun.In(accountIDs)).Where("last_seen_at IS NOT NULL").Scan(ctx)
	for _, a := range accounts {
		seen[a.ID] = *a.LastSeenAt
	}
	return seen, err
}
