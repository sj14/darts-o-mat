package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/uptrace/bun"
)

// lastSettings are the settings of an account's last new game or tournament of a mode,
// which the start form shows again. They are JSON (game.Settings with its rules), not
// columns like the settings of games: nothing filters by them, and a new mode needs no
// table for them.
type lastSettings struct {
	bun.BaseModel `bun:"table:last_settings"`

	AccountID int64     `bun:",pk"`
	Mode      game.Mode `bun:",pk"`
	Settings  string    `bun:",notnull"`
}

// LastSettings returns the settings of the account's last new game or tournament of the
// mode; ok is false if it created none yet.
func (store *Store) LastSettings(ctx context.Context, accountID int64, mode game.Mode) (s game.Settings, ok bool, err error) {
	var row lastSettings
	err = store.db.NewSelect().Model(&row).Where("account_id = ? AND mode = ?", accountID, mode).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Settings{}, false, nil
	}
	if err != nil {
		return game.Settings{}, false, err
	}
	s.Rules = game.NewRules(mode)
	if err := json.Unmarshal([]byte(row.Settings), &s); err != nil {
		return game.Settings{}, false, err
	}
	return s, true, nil
}

// SetLastSettings stores s as the settings of the account's last new game or tournament
// of its mode.
func (store *Store) SetLastSettings(ctx context.Context, accountID int64, s game.Settings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	row := lastSettings{AccountID: accountID, Mode: s.Mode, Settings: string(data)}
	_, err = store.db.NewInsert().Model(&row).On("CONFLICT (account_id, mode) DO UPDATE").Set("settings = EXCLUDED.settings").Exec(ctx)
	return err
}
