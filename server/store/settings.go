package store

import (
	"context"
	"fmt"

	"github.com/sj14/darts-o-mat/server/domain/game"
	"github.com/sj14/darts-o-mat/server/domain/game/aroundtheclock"
	"github.com/sj14/darts-o-mat/server/domain/game/x01"
	"github.com/uptrace/bun"
)

// Every game mode keeps its settings in a table of its own, so a new mode needs a table and
// no column in games and tournaments. A game or tournament points to its row with
// settings_id (together with its mode, so there is no foreign key); the games of a
// tournament share the row of the tournament.
//
// To add a mode: a row type and an entry in modeTables below (and the mode in
// game/modes.go).

// modeTable is how the settings of one mode are stored and found again.
type modeTable struct {
	table string
	// save inserts the settings of s and returns the id of their row.
	save func(ctx context.Context, db bun.IDB, s *game.Settings) (int64, error)
	// load fills in the settings of those that point to a row, by the row's id.
	load func(ctx context.Context, db bun.IDB, by map[int64][]*game.Settings) error
}

var modeTables = map[game.Mode]modeTable{
	game.ModeX01: {
		table: "game_settings_x01",
		save: func(ctx context.Context, db bun.IDB, s *game.Settings) (int64, error) {
			row := x01Row{Settings: *s.Rules.(*x01.Settings)}
			_, err := db.NewInsert().Model(&row).Exec(ctx)
			return row.ID, err
		},
		load: func(ctx context.Context, db bun.IDB, by map[int64][]*game.Settings) error {
			var rows []x01Row
			if err := selectRows(ctx, db, &rows, by); err != nil {
				return err
			}
			for _, r := range rows {
				for _, s := range by[r.ID] {
					settings := r.Settings
					s.Rules = &settings
				}
			}
			return nil
		},
	},
	game.ModeAroundTheClock: {
		table: "game_settings_around_the_clock",
		save: func(ctx context.Context, db bun.IDB, s *game.Settings) (int64, error) {
			row := aroundTheClockRow{Settings: *s.Rules.(*aroundtheclock.Settings)}
			_, err := db.NewInsert().Model(&row).Exec(ctx)
			return row.ID, err
		},
		load: func(ctx context.Context, db bun.IDB, by map[int64][]*game.Settings) error {
			var rows []aroundTheClockRow
			if err := selectRows(ctx, db, &rows, by); err != nil {
				return err
			}
			for _, r := range rows {
				for _, s := range by[r.ID] {
					settings := r.Settings
					s.Rules = &settings
				}
			}
			return nil
		},
	},
}

// The rows: the mode's own settings with an id.
type x01Row struct {
	bun.BaseModel `bun:"table:game_settings_x01"`

	ID int64 `bun:",pk,autoincrement"`
	x01.Settings
}

type aroundTheClockRow struct {
	bun.BaseModel `bun:"table:game_settings_around_the_clock"`

	ID int64 `bun:",pk,autoincrement"`
	aroundtheclock.Settings
}

// selectRows loads the rows with the ids of by.
func selectRows(ctx context.Context, db bun.IDB, rows any, by map[int64][]*game.Settings) error {
	ids := make([]int64, 0, len(by))
	for id := range by {
		ids = append(ids, id)
	}
	return db.NewSelect().Model(rows).Where("id IN (?)", bun.In(ids)).Scan(ctx)
}

// saveSettings stores the mode's settings of a new game or tournament and sets its
// SettingsID, unless it has one already: the games of a tournament use the tournament's.
func saveSettings(ctx context.Context, db bun.IDB, s *game.Settings) error {
	if s.SettingsID != 0 {
		return nil
	}
	table, ok := modeTables[s.Mode]
	if !ok {
		return fmt.Errorf("no settings table for mode %q", s.Mode)
	}
	id, err := table.save(ctx, db, s)
	s.SettingsID = id
	return err
}

// loadSettings fills in the mode's settings of games or tournaments that were read.
func loadSettings(ctx context.Context, db bun.IDB, all []*game.Settings) error {
	byMode := map[game.Mode]map[int64][]*game.Settings{}
	for _, s := range all {
		if byMode[s.Mode] == nil {
			byMode[s.Mode] = map[int64][]*game.Settings{}
		}
		byMode[s.Mode][s.SettingsID] = append(byMode[s.Mode][s.SettingsID], s)
	}
	for mode, by := range byMode {
		table, ok := modeTables[mode]
		if !ok {
			return fmt.Errorf("no settings table for mode %q", mode)
		}
		if err := table.load(ctx, db, by); err != nil {
			return err
		}
		// A row that is gone (e.g. a database of an older version) is an error, not a game
		// without rules.
		for id, settings := range by {
			if settings[0].Rules == nil {
				return fmt.Errorf("the %s settings %d are missing (a database of an older version? delete it)", mode, id)
			}
		}
	}
	return nil
}

// deleteSettings removes the row of settings whose game was deleted, unless a game or a
// tournament still uses it (a tournament's games share its row).
func deleteSettings(ctx context.Context, db bun.IDB, s game.Settings) error {
	table, ok := modeTables[s.Mode]
	if !ok {
		return fmt.Errorf("no settings table for mode %q", s.Mode)
	}
	_, err := db.NewDelete().TableExpr(table.table).Where("id = ?", s.SettingsID).
		Where("NOT EXISTS (SELECT 1 FROM games WHERE mode = ? AND settings_id = ?)", s.Mode, s.SettingsID).
		Where("NOT EXISTS (SELECT 1 FROM tournaments WHERE mode = ? AND settings_id = ?)", s.Mode, s.SettingsID).
		Exec(ctx)
	return err
}
