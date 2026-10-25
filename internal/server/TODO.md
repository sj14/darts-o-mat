# TODO: darts-o-mat (server)

Open tasks of the [server](../../cmd/darts-o-mat/README.md). Tasks of the whole project: [TODO.md](../../TODO.md).

## Game page

- Instead/Additional of a keyboard, click into the simulated/drawn board.
  - Maybe hide the keypad by default and when you click in the board, it opens or you can click a second time into the board for the position.

## Game modes

- Game mode segment training (planned 2026-10-14, why `dartboard.Segment` knows the inner and outer single field): e.g. hit a given segment as often as possible. A new mode follows [AGENTS.md → game state](AGENTS.md#game-state).
- Around the clock where only doubles or only triples count (wished 2026-10-24): a setting of the mode (migration for both databases), the hit rule, the start form, a hint on the keypad; roughly 80–120 lines. Its results are another discipline for the form in the list of open games, see [AGENTS.md → open games](AGENTS.md#open-games): the stored result has to know the variant.

## Games

- Completing a game is built (2026-10-18, reworked 2026-10-20, see [AGENTS.md → completing a game](AGENTS.md#game-state)): no button, a game is completed when a player goes on to another game or tournament, or after a minute. What is left: a "Revanche" button on the banner (the "Neues Spiel" button was removed from it on 2026-10-18; the logo in the header leads home); the pages of a tournament and of a game that nobody has open aren't refreshed when the minute without a change passes (nothing runs then, the next look or click shows it), e.g. a short `hx-trigger` refresh of the tournament page while a result can still be corrected (to talk about with the user, 2026-10-20). Decided: no timer or background job, the minute is worked out when the game is looked at.
- One seat per player is built (2026-10-22, see [AGENTS.md → accounts and players](AGENTS.md#accounts-and-players)): refused with a link. Left out for now: an "abort it and start the new one" offer (leaving a game with others counts in "left early").
- Lobby as the one way in is built (2026-10-22, see [AGENTS.md → lobby](AGENTS.md#lobby)). Set aside: the game's settings in the lobby instead of the start form (asked 2026-10-22, put off by the user to finish the lobby first; open question: who may change them until when, e.g. only while no other account is in, so strangers aren't surprised); reordering the players in the lobby (the order is drawn at the start for now).
- Allow others to watch games if they know the link.
- Play with strangers, what is left (open games, a board per player, leaving, the optional captcha, the limit on failed logins, the form of the players with a filter by it and the quick match whose allowed difference grows with the wait are built, the last three on 2026-10-24 instead of an ELO rating, see [AGENTS.md → open games](AGENTS.md#open-games)): board-only lobbies (built 2026-10-20) don't pass the rule on to a tournament's games, see [AGENTS.md → open games](AGENTS.md#open-games), rate limits for creating open games and quick matches.
- Time per turn (built 2026-10-18, see [AGENTS.md → game state](AGENTS.md#game-state)); set aside 2026-10-24, about 150–200 lines are too much for it for now: show skipped turns and players who left because of it in the history (today a skipped turn has no row, or a "…" after its darts); needs a `Skipped` field per turn in each mode. Maybe a warning on the player's card after the first skipped turn.

## Tournaments

- Tournaments (base built 2026-10-14, see [AGENTS.md → tournaments](AGENTS.md#tournaments)):
  - Groups + knockout: groups as round robin, the best N per group into a bracket (group winners against runners-up of other groups). Reuses table and bracket; new: group assignment, qualification, seeding from the groups, stage transition, page with several tables. Effort roughly a third of the base.
  - Final (or last round) with its own legs/sets.

## Statistics

- Heatmap: choose a period (game mode since 2026-10-20), a finer drawing (smooth instead of squares), and the hits per field as numbers; maybe also show which fields were corrected (position and field differ, see [statistics](AGENTS.md#statistics)).
- Statistics at the end: the game page shows the statistics of the whole game below the history (table, and a heatmap per player for camera darts, since 2026-10-18; a separate statistics page was rejected as it would repeat the table). Still open: the statistics of a single leg after it, and a comparison of the players leg by leg.
- Statistics from the table (`game_stats`) allow trends over time (average per week, last 30 days): not built, needs a date per result (`games.completed_at` or the game's start).

## Boards and cameras

- Plausibility check for camera darts while waiting for the takeout (discussed 2026-10-15, not built): a camera dart then records the takeout first (implicit takeout, `service.AddBoardDart`); a dart that comes too soon after the third one (e.g. under a few seconds, nobody pulled the darts out yet) is more likely a misdetection and could be refused or asked about.

## Accounts and guests

- Admin area: built with the imprint and the privacy policy (2026-10-22), opening and closing the registration (2026-10-23) and several admins (2026-10-24), see [AGENTS.md → admin and pages](AGENTS.md#admin-and-pages). Left: the pages and the admins in the JSON API (not needed so far).

## Technical

- Mail sending (e.g. password reset, confirm the email): the optional email of an account is stored, unique and changeable already, but not confirmed and nothing is sent ([AGENTS.md → email](AGENTS.md#deleting-guests-and-accounts)). Like the captcha it has to stay optional (SMTP settings by flag or environment, nothing without them), see the [root document](../../AGENTS.md#from-the-home-server-to-saas). Decided 2026-10-18: no single flag for a "public" setup; every feature for the big case is switched on by itself.
- Failed logins: a list of trusted proxies or the number of hops for `X-Forwarded-For` chains (today the last value of one header counts), and a captcha only after failures, see [AGENTS.md → failed logins](AGENTS.md#failed-logins).
- A captcha for logging in was left out on purpose (2026-10-18): every login would pay for it and the API couldn't use it; the limit on failed logins does that job.
- Several replicas of the server: built 2026-10-25 (aim set 2026-10-24), see [AGENTS.md → replicas](AGENTS.md#replicas). Left, only if needed:
  - **Two stages of locks**, only if the global lock turns out too slow (`slowLock` logs when it is held long): darts, undo, corrections, takeouts and `endTurn` take a lock per game (`pg_advisory_lock(game_id)`, in memory a mutex per game), so the frequent changes of different games run in parallel; the seat operations (create, join, accept, start a match, leave, delete) keep the global lock, since they check one seat per player across games, and are rare. The catch: some seat operations also change other games (`completeFinished` completes old games, leaving adds entries), which must not run at the same time as an undo in that game. They take the global lock and then the locks of the games they touch, always in the same order (global first, then by game ID). Considerably more than swapping `s.mu`.
  - **Other services** (optional, each switched on by a flag and an environment variable like the captcha; only when PostgreSQL measurably doesn't keep up; each one a new library and a service for the operator to run, so ask first). Examples: Redis, or NATS with JetStream's key-value store. Each concern has an interface of its own already (`service.Bus`, `Locker`, `LoginFailures`, `LastSeen`, `ServerStart`, set in `Options`), not one interface for a whole "backend"; a new service implements one of them. Live updates: Redis Pub/Sub and NATS subjects both fit as they are. Login failures, presence, the start: Redis keys with an expiry or a NATS KV bucket with a TTL fit (presence especially: a key per account that expires after 5 minutes). The lock is the weak point of both: a lock in Redis or NATS KV (create the key only if it doesn't exist) needs an expiry and has to be renewed while it is held, since it isn't bound to a connection the way an advisory lock is. A replica that stalls longer than the expiry (GC pause, network) can then write at the same time as another one, and the database doesn't know about the lock. So the lock stays in PostgreSQL, also when the rest moves elsewhere.
- Unsure: switch the primary IDs to UUIDs.
- Database migrations are built (2026-10-22, see [AGENTS.md → database](AGENTS.md#database)). Left:
  - Alternative if writing the SQLite half by hand gets too much: [Atlas](https://atlasgo.io) (`migrate diff` for both databases, also rebuilds SQLite tables), a new external tool. Switch to Bun's own once `sqlitedialect` has a schema inspector.
  - Watch [Turso](https://github.com/tursodatabase/turso): its Postgres frontend (dialect and wire protocol) would let SQLite files be used with only the Postgres dialect in the code, `pgdriver` and `AutoMigrator` for both. Not yet (2026-10-17): the frontend is listed as experimental, Turso isn't 1.0, and over the wire protocol it is a server process of its own, which the home installation would have to install and run next to darts-o-mat. Once it is stable, test whether `AutoMigrator` works with it (it reads the schema from Postgres' system catalogs).
  - Rejected for now: dropping SQLite (the home installation would need a Postgres server or a container) and [embedded-postgres](https://github.com/fergusstrange/embedded-postgres) for it (downloads the Postgres binaries on the first start, against "works without internet" unless bundled with tens of MB per platform; v2 is an alpha meant mainly for tests; a second process to manage). It would fit the tests, if SQLite is ever dropped.
- ELO rating (offered 2026-10-20, after the thumbs; set aside 2026-10-24: to find opponents of about the same strength, the average is to be used instead, see [play with strangers](#games)): per mode, one value per player with an account, only for games of exactly two such players of different accounts (no guests, no games with more players at first), start 1000, K=32; stored per game (before and after) as the result is stored, because the order matters. Starting or joining the next game or tournament completes the last one (2026-10-20), so the rating is final before it; only a game left alone is completed lazily by the minute. Caveats to name: no allowance for 301 against 501, legs or sets; friends can pump each other up.
