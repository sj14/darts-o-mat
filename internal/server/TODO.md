# TODO: darts-o-mat (server)

Open tasks of the [server](../../cmd/darts-o-mat/README.md). Tasks of the whole project: [TODO.md](../../TODO.md).

## Game page

- Instead/Additional of a keyboard, click into the simulated/drawn board.
  - Maybe hide the keypad by default and when you click in the board, it opens or you can click a second time into the board for the position.

## Game modes

- Game mode segment training (planned 2026-10-14, why `dartboard.Segment` knows the inner and outer single field): e.g. hit a given segment as often as possible. A new mode follows [AGENTS.md → game state](AGENTS.md#game-state).

## Games

- Confirming the end of a game is built (2026-10-18, see [AGENTS.md → completing a game](AGENTS.md#game-state)). What is left: a "Revanche" button on the banner (the "Neues Spiel" button was removed from it on 2026-10-18; the logo in the header leads home); the pages of a tournament and of a game that nobody has open aren't refreshed when the minute without a change passes (nothing runs then, the next look or click shows it), which a short `hx-trigger` refresh of the tournament page while a result is unconfirmed would fix; the confirmation is only by button, by starting a game and by that minute, not with a longer wait or a timer (decided: no timer, the minute is worked out when the game is looked at).
- Option, undecided (asked 2026-10-18, advised against: it barely simplifies the code, since leaving, guests, tournaments and boards still need their cases, and it adds blocking edge cases): allow only one running game per account (games with a player of it, including one waiting in the lobby).
  - For: matches reality (one person plays one game at a time), no forgotten games, and the board warning/moving (see [boards and cameras](#boards-and-cameras)) becomes unnecessary, because a board could then only be in its owner's single game.
  - Against: club or pub use – one account couldn't run guest-only games on two boards at the same time.
  - If done: starting a new game while one runs offers "continue it" or "abort it and start the new one" instead of just refusing; invitations stay open until the current game is finished or aborted.
- Allow others to watch games if they know the link.
- Play with strangers, what is left (open games, a board per player, leaving, the optional captcha and the limit on failed logins are built, see [AGENTS.md → open games](AGENTS.md#open-games)): board-only lobbies (built 2026-10-20) still let a board be taken by a newer game and don't pass the rule on to a tournament's games, see [AGENTS.md → open games](AGENTS.md#open-games), quick match (join the oldest open game with default rules or open one and wait), rate limits for creating open games.
- Time per turn (built 2026-10-18, see [AGENTS.md → game state](AGENTS.md#game-state)): show skipped turns and players who left because of it in the history (today a skipped turn has no row, or a "…" after its darts); needs a `Skipped` field per turn in each mode. Maybe a warning on the player's card after the first skipped turn.

## Tournaments

- Tournaments (base built 2026-10-14, see [AGENTS.md → tournaments](AGENTS.md#tournaments)):
  - Groups + knockout: groups as round robin, the best N per group into a bracket (group winners against runners-up of other groups). Reuses table and bracket; new: group assignment, qualification, seeding from the groups, stage transition, page with several tables. Effort roughly a third of the base.
  - Final (or last round) with its own legs/sets.

## Statistics

- Heatmap: choose a period (game mode since 2026-10-20), a finer drawing (smooth instead of squares), and the hits per field as numbers; maybe also show which fields were corrected (position and field differ, see [statistics](AGENTS.md#statistics)).
- Statistics at the end: the game page shows the statistics of the whole game below the history (table, and a heatmap per player for camera darts, since 2026-10-18; a separate statistics page was rejected as it would repeat the table). Still open: the statistics of a single leg after it, and a comparison of the players leg by leg.
- `service.Stats` loads every game's and tournament's participants (`ListGames`, `ListTournaments`) on each visit; a query only for those without stored results would do.
- Statistics from the table (`game_stats`) allow trends over time (average per week, last 30 days): not built, needs a date per result (`games.completed_at` or the game's start).

## Boards and cameras

- Plausibility check for camera darts while waiting for the takeout (discussed 2026-10-15, not built): a camera dart then records the takeout first (implicit takeout, `service.AddBoardDart`); a dart that comes too soon after the third one (e.g. under a few seconds, nobody pulled the darts out yet) is more likely a misdetection and could be refused or asked about.

## Accounts and guests

- Add admin area

## Technical

- Mail sending (e.g. password reset, confirm the email): the optional email of an account is stored, unique and changeable already, but not confirmed and nothing is sent ([AGENTS.md → email](AGENTS.md#deleting-guests-and-accounts)). Like the captcha it has to stay optional (SMTP settings by flag or environment, nothing without them), see the [root document](../../AGENTS.md#from-the-home-server-to-saas). Decided 2026-10-18: no single flag for a "public" setup; every feature for the big case is switched on by itself.
- Failed logins: a list of trusted proxies or the number of hops for `X-Forwarded-For` chains (today the last value of one header counts), and a captcha only after failures, see [AGENTS.md → failed logins](AGENTS.md#failed-logins).
- A captcha for logging in was left out on purpose (2026-10-18): every login would pay for it and the API couldn't use it; the limit on failed logins does that job.
- Unsure: switch the primary IDs to UUIDs.
- Database migrations, once existing data has to be kept (until then the schema comes from the structs, `CreateTable … IfNotExists`, and breaking changes mean deleting the database). Decided 2026-10-17 (SQLite is the database of the home installation, so it needs migrations too):
  - PostgreSQL: generate them with `migrate.AutoMigrator` from `github.com/uptrace/bun/migrate` (`CreateSQLMigrations`: compares the structs with a database in the previous state and writes the difference as SQL files), review and commit them.
  - SQLite: write the same migration by hand, as long as Bun's `sqlitedialect` has no schema inspector and migrator (checked in v1.3.0: only `pgdialect` has them, `NewAutoMigrator` fails for SQLite). Many changes need a table rebuild there, since SQLite can't alter columns. Switch to the generated ones once Bun supports SQLite.
  - One migration directory per database (e.g. `internal/server/store/migrations/postgres` and `…/sqlite`, same names), embedded with `go:embed` and applied at start with `bun/migrate`, instead of `CreateTable`.
  - Renames aren't detected (they come out as drop + add, losing the data), and data migrations (e.g. filling a new required column): fix them by hand in the generated file. A test could apply all migrations to an empty database of each kind and compare the result with the structs, so the two directories can't drift apart.
  - Alternative if the hand-written half gets too much: [Atlas](https://atlasgo.io) (`migrate diff` for both databases, also rebuilds SQLite tables), a new external tool.
  - Watch [Turso](https://github.com/tursodatabase/turso): its Postgres frontend (dialect and wire protocol) would let SQLite files be used with only the Postgres dialect in the code, `pgdriver` and `AutoMigrator` for both. Not yet (2026-10-17): the frontend is listed as experimental, Turso isn't 1.0, and over the wire protocol it is a server process of its own, which the home installation would have to install and run next to darts-o-mat. Once it is stable, test whether `AutoMigrator` works with it (it reads the schema from Postgres' system catalogs).
  - Rejected for now: dropping SQLite (the home installation would need a Postgres server or a container) and [embedded-postgres](https://github.com/fergusstrange/embedded-postgres) for it (downloads the Postgres binaries on the first start, against "works without internet" unless bundled with tens of MB per platform; v2 is an alpha meant mainly for tests; a second process to manage). It would fit the tests, if SQLite is ever dropped.
