# TODO: darts-o-mat (server)

Open tasks of the [server](README.md). Tasks of the whole project: [TODO.md](../../TODO.md).

## Game page

- Instead/Additional of a keyboard, click into the simulated/drawn board.
  - Maybe hide the keypad by default and when you click in the board, it opens or you can click a second time into the board for the position.

## Game modes

- Game mode segment training (planned 2026-10-14, why `throw.Segment` knows the inner and outer single field): e.g. hit a given segment as often as possible. A new mode follows [AGENTS.md → game state](AGENTS.md#game-state).

## Games

- Confirm the end of a game: after the last dart the game is over but not yet completed; the players check it (e.g. correct a misdetected final dart from the camera) and confirm. Afterwards it can't be undone or corrected anymore.
  - Confirm automatically when one of the players starts a new game, maybe also after some time, so a forgotten confirmation doesn't leave games hanging.
  - Adds a state (running → over → completed) that undo, corrections, the camera and the "one running game per account" option have to respect (an unconfirmed game still counts as running).
  - Natural place for buttons like "Revanche" or "Neues Spiel".
- Option: let every player with an account leave a running game, not only the host abort it.
  - With 2 players, leaving aborts the game (as today).
  - With more than 2 players, the game continues without the player who left.
  - Note: the game state is computed by replaying the darts with a fixed player list, so leaving has to become an entry in the log (like the takeout), from which on the replay skips that player. Open questions: what happens to their darts and statistics, can the host also leave (who is host then), and can the host remove guests the same way.
- Option, undecided: allow only one running game per account (games it hosts or has accepted, including one waiting in the lobby).
  - For: matches reality (one person plays one game at a time), no forgotten games, and the board warning/moving (see [boards and cameras](#boards-and-cameras)) becomes unnecessary, because a board could then only be in its owner's single game.
  - Against: club or pub use – one account couldn't run guest-only games on two boards at the same time.
  - If done: starting a new game while one runs offers "continue it" or "abort it and start the new one" instead of just refusing; invitations stay open until the current game is finished or aborted.
- Allow others to watch games if they know the link.
- Play with strangers (idea, not designed yet), for games and tournaments; inviting is shared (`service.invitedPlayers`, today friends only). For example:
  - When creating a game, tick something like "allow strangers"; the game waits in the lobby for them.
  - Finding a game: either a list of open games to join, or automatic matchmaking.
  - Today only friends can be invited, and games are hidden from everyone else (404); this would be the first exception.

## Tournaments

- Tournaments (base built 2026-10-14, see [AGENTS.md → tournaments](AGENTS.md#tournaments)):
  - Groups + knockout: groups as round robin, the best N per group into a bracket (group winners against runners-up of other groups). Reuses table and bracket; new: group assignment, qualification, seeding from the groups, stage transition, page with several tables. Effort roughly a third of the base.
  - JSON API for tournaments.
  - Update the tournament page live (SSE like the game page).
  - Final (or last round) with its own legs/sets.

## Statistics

- Statistics: add a heatmap of the dart positions.
- Show a statistics summary at the end: after a leg the statistics of that leg, after the game the statistics of the whole game (per player, e.g. average, highest visit, checkout, darts). Open: show the game summary as soon as the game is over (together with the confirmation, see [games](#games)) or only once it is completed.
- Statistics of the durations (`turn.Times`, since 2026-10-16 only shown on the game and tournament page): e.g. average leg duration per player and mode.
- Later, only once the statistics page gets measurably slow (builds on confirmed game ends): a statistics table per player and game mode with running totals, updated when a game is completed. Safe because completed games can't change anymore; sums just add up and highest/best values only need max/min. The replay stays for rebuilding the table when statistics are added or fixed. A cheaper first step: load the darts of all games in one query instead of one per game; also `service.Stats` replays each tournament's games a second time (`tournamentView`) only to learn the tournament's winner.

## Boards and cameras

- Boards (cameras):
  - During a game, allow switching from no board to a board; this also updates the board's "last used" time (`board.Board.LastUsedAt`, so far only set when a game is created on it).
  - A board belongs to at most one game at a time. If the selected board is already used by another game, show a warning but allow it; the board then moves to this game and the other game continues without a board. This replaces today's rule that camera darts go to the newest active game with the board.
- Plausibility check for camera darts while waiting for the takeout (discussed 2026-10-15, not built): a camera dart then records the takeout first (implicit takeout, `service.AddBoardDart`); a dart that comes too soon after the third one (e.g. under a few seconds, nobody pulled the darts out yet) is more likely a misdetection and could be refused or asked about.

## Accounts and guests

- Do not allow the same guest name twice (`service.CreateGuest` only refuses an empty name).
- Delete guests and delete the own account (undecided how). Always deleted: name, login, sessions, friendships, boards with camera tokens, the account's guests. Open:
  - Past games with the deleted player: keep them with the player shown as "Gelöschter Spieler" (players and their history stay intact; recommended) or delete them with all darts (fellow players, also friends, lose them and their statistics). Removing just the player from a game isn't possible: darts aren't linked to players, who threw is derived from the throwing order when replaying.
  - Guest in a running game: refuse ("finish or abort it first"; recommended) or abort the game automatically.
  - Account in running games (own ones and accepted ones of friends): abort them automatically and decline open invitations (recommended; refusing would block until friends finish, since only the host can abort) or refuse.
  - Deleting the account should ask for the password again.
- Add admin area

## Technical

- Unsure: switch the primary IDs to UUIDs.
- Database migrations, once existing data has to be kept (until then the schema comes from the structs, `CreateTable … IfNotExists`, and breaking changes mean deleting the database). Decided 2026-10-17 (SQLite is the database of the home installation, so it needs migrations too):
  - PostgreSQL: generate them with `migrate.AutoMigrator` from `github.com/uptrace/bun/migrate` (`CreateSQLMigrations`: compares the structs with a database in the previous state and writes the difference as SQL files), review and commit them.
  - SQLite: write the same migration by hand, as long as Bun's `sqlitedialect` has no schema inspector and migrator (checked in v1.3.0: only `pgdialect` has them, `NewAutoMigrator` fails for SQLite). Many changes need a table rebuild there, since SQLite can't alter columns. Switch to the generated ones once Bun supports SQLite.
  - One migration directory per database (e.g. `pkg/repos/store/migrations/postgres` and `…/sqlite`, same names), embedded with `go:embed` and applied at start with `bun/migrate`, instead of `CreateTable`.
  - Renames aren't detected (they come out as drop + add, losing the data), and data migrations (e.g. filling a new required column): fix them by hand in the generated file. A test could apply all migrations to an empty database of each kind and compare the result with the structs, so the two directories can't drift apart.
  - Alternative if the hand-written half gets too much: [Atlas](https://atlasgo.io) (`migrate diff` for both databases, also rebuilds SQLite tables), a new external tool.
  - Watch [Turso](https://github.com/tursodatabase/turso): its Postgres frontend (dialect and wire protocol) would let SQLite files be used with only the Postgres dialect in the code, `pgdriver` and `AutoMigrator` for both. Not yet (2026-10-17): the frontend is listed as experimental, Turso isn't 1.0, and over the wire protocol it is a server process of its own, which the home installation would have to install and run next to darts-o-mat. Once it is stable, test whether `AutoMigrator` works with it (it reads the schema from Postgres' system catalogs).
  - Rejected for now: dropping SQLite (the home installation would need a Postgres server or a container) and [embedded-postgres](https://github.com/fergusstrange/embedded-postgres) for it (downloads the Postgres binaries on the first start, against "works without internet" unless bundled with tens of MB per platform; v2 is an alpha meant mainly for tests; a second process to manage). It would fit the tests, if SQLite is ever dropped.
