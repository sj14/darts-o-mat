# darts-o-mat (server)

The web server: web interface, JSON API, accounts, games, game state and statistics.
Project-wide rules, the shared stack and how server and camera work together: [root AGENTS.md](../../AGENTS.md). For humans: [README.md](README.md). Open tasks: [TODO.md](TODO.md). The camera side: [darts-o-cam](../darts-o-cam/AGENTS.md).

## Requirements

- **Multiple accounts with login** (implemented):
  - An account belongs to exactly one player (created at registration) and can have several boards.
  - Guest players have no account; they belong to the account that created them (`player.GuestOf`), and only that account can add them to games.
  - Friend list: friendship is mutual (request, the other side accepts; if both send each other a request, it counts as accepted). Removing ends the friendship, running games together stay.
  - Only friends can be invited to games. They are invited by username and have to accept. As long as an invitation is open, the game is in the lobby and doesn't accept darts. Declining or uninviting removes the player from the game; a game without players is deleted.
  - The host and accepted accounts may enter/undo darts. Whoever isn't part of a game gets `ErrNotFound` (404), so its existence isn't revealed.
  - Registration is open, can be turned off with `-registration=false`.

## Structure

- `cmd/darts-o-mat`: starts the server (flags `-db`, `-dsn`, `-addr`, `-registration`)
- `pkg/api`: JSON API under `/api`, see [API](#api)
- `pkg/service`: application logic including permission checks (accounts, guests, boards, games, invitations, darts), shared by API and web interface. New logic belongs here, not in the handlers; methods receive the logged-in account.
- `pkg/auth`: session cookie (HttpOnly, SameSite=Lax) and middleware that puts the logged-in account into the request context. The database only stores the SHA-256 hash of the session token, passwords as bcrypt hash.
- `pkg/domain`: domain logic (board, players, darts, game modes X01 and Around the Clock, tournaments), see [game state](#game-state) and [tournaments](#tournaments)
- `pkg/frontend`: web interface: controllers, templ views (`view`) and embedded static files (`static`)
- `pkg/repos/store`: persistence; one shared Bun store for both databases, only the connection setup (`NewPostgres`/`NewSQLite`) differs
- `Containerfile` (project root, 2026-10-14): image of the server only. Build stage `golang:1` (user's choice, 2026-10-14: the latest Go 1.x; should `go.mod` ever need a newer one, the go command fetches it itself) on `$BUILDPLATFORM`, cross-compiled with `TARGETOS`/`TARGETARCH` (multi-platform without emulation), `CGO_ENABLED=0`; runs in `gcr.io/distroless/static-debian13:nonroot` (user's wish: distroless) – possible because the server has no cgo (pure Go SQLite) and reads its templates and static files from the binary. `/data` belongs to `nonroot` (a new volume takes over its owner) and is the working directory, so the default `-dsn file:darts-o-mat.db` lands in the volume and flags can be given without repeating it. `.dockerignore` keeps `.git`, databases and build outputs out of the context. darts-o-cam has no image: see [camera TODO](../darts-o-cam/TODO.md). Tested 2026-10-14 with Docker (Rancher Desktop): page, CSS and the database in the volume.

## Database

- Boards store when a game was last started on them (`board.Board.LastUsedAt`, set in the same transaction as the game in `store.CreateGame`); the start form preselects that board (`view.lastUsedBoard`).

- PostgreSQL and SQLite are both supported (chosen via `-db sqlite|postgres` and `-dsn`). Persistence code must work with both databases.
- ORM/query builder: [Bun](https://bun.uptrace.dev) (`pgdialect`/`pgdriver` for PostgreSQL, `sqlitedialect` with the cgo-free driver `modernc.org/sqlite` for SQLite)

## Game page

Shared design rules (Tailwind, building blocks, performance): [root AGENTS.md → technologies](../../AGENTS.md#technologies).

- Game page in `game.templ`, board SVG in `board.go`. In Around the Clock the current player's target is highlighted (`targetMark`): the rest of the playing area is dimmed (`dimPath`, black at 60 % with a hole for the target, fill-rule evenodd), the target keeps its own colors with an amber outline – the whole segment, because single/double/triple count – plus the amber number; the small bull additionally with a ring; on top a glow layer with a wide amber border that only pulses its opacity via `animate-target`; `boardTargetOf`, not during the bull-off. (Until 2026-10-13 the segment was covered with amber at 45 %, which hardly showed on the cream fields.)
- **Game page without scrolling** (user's wish 2026-10-12): from `lg` three columns – players stacked (the current one large as `playerCard`, below the others in throwing order as `playerRow`, the next one with "als Nächstes"; `playerLines` rotates the list, after the game the winner is on top), middle the darts of the visit and the input, right the board; history and statistics below. Goal: everything for one throw fits into 1366×650 (laptop with browser bar), therefore smaller keypad keys (`lg:h-12`) and dart tiles (`lg:h-20`) on `lg`; between `lg` and `xl` the compact rows don't show the secondary number, otherwise the names get cut off. Narrow screens stack the columns.
- Keypad (`keypad`): the keys follow the chosen multiplier with CSS only (`group/keypad` with `group-has-[[value='2']:checked]/keypad:…`, no JavaScript, no request): D1…D20 and "Bullseye" for double, T1…T20 for triple, where the bull is greyed out and not clickable (there is no triple bull; the server rejects it anyway, `throw.ErrInvalid`).
- **Misses the cameras located** show where they missed (user's wish 2026-10-14): "M7" for a miss beside the 7 (`throw.Dart.String`). darts-o-cam scores the dart and sends the number with the miss (`dartboard.Score`: the number whose segment lines, drawn on, the miss lies between), like it sends the multiplier for a hit; `normalizeDart` keeps it, `Validate` allows 0–20 for a miss. Misses entered by hand or corrected to "Daneben" have number 0 and show "Daneben". Points, statistics and the game modes only look at `Miss`.
- Around the Clock enters darts via buttons for the current target ("Treffer" without the number, which the board and the player card show – "Treffer Bull" was cramped; with the skip option also D7/T7, plus "Daneben"; `dartInput`), the full keypad is collapsed below in a `<details>`; correcting always uses the full keypad.

## Sounds

`static/sounds.js` generates the sounds with the Web Audio API (no audio files) – the only custom JavaScript file, because htmx can't play sounds.

- The server marks `#board` with `data-sound` (kind of the last log entry: dart, double, triple, bull, hit, miss, bust, 180, win, takeout; `view.soundKind`; in Around the Clock every dart off the target is `miss`, not only "Daneben") and `data-sound-id` (its ID). A sound plays on every swap of the board if the ID is greater than before – so for darts from this device, other devices and the camera; if the ID is smaller, something was undone (own sound `undo`). Loading and correcting (same ID) make no sound. On/off in the header of the game page, stored in `localStorage`.
- Browsers only play after an interaction with the page (autoplay block; click, tap or key – `pointerdown`/`keydown` unlock, not only `click`). Until then the `AudioContext` is `suspended` and sounds get lost – exactly in camera games on a freshly loaded page where nobody touches the page (noticed by the user 2026-10-12). Therefore the page already creates the context on load and the button then shows "🔈 Für Ton klicken" (amber); clicking it only unlocks instead of turning off (`unlocking`, because `pointerdown` comes before the `click`). The block can't be turned off from the page (only via browser setting, e.g. Firefox "Allow audio and video autoplay" for the site).
- The built-in browser of the Claude app (Electron) has no block; for testing, inject an `AudioContext` that stays silent until `navigator.userActivation`.

## Camera connection

How server and camera work together in general: [root AGENTS.md](../../AGENTS.md#how-server-and-camera-work-together). The client side: [darts-o-cam → connection to the game](../darts-o-cam/AGENTS.md#connection-to-the-game).

- One camera token per board (boards page or `POST /api/boards/{id}/token`), stored only as SHA-256 hash in the table `tokens` (`board.Token`). `darts-o-cam` sends `Authorization: Bearer <token>` to `POST /api/board/darts` and `/api/board/takeout`. The dart goes to the newest game with this board that is ready and not over (`service.AddBoardDart`), on behalf of the board owner – so same rules and same undo as typing it in. No such game: `ErrNoActiveGame` (409).
- The dart is scored by darts-o-cam (`throw.Dart` with number, multiplier, miss and X/Y in mm); for a miss `number` is the number it went beside (0: unknown), see [game page](#game-page).
- **Takeout** (`service.BoardTakeout`): if the visit is incomplete at that point, the missing darts didn't stick (bounce-outs, misses) and are entered as "Daneben"; then the takeout is logged and the next player is up.
- **Live display via server-sent events:** the service reports every change to a game (`Service.Subscribe`/`notify`: dart, undo, invitation answered – whether via web interface, API or camera). The game page connects to `GET /games/{id}/events` (htmx SSE extension, vendored in `static`); on every notification the server sends the board rendered for the respective viewer as event `board`, and the element `#board` swaps itself with it (`sse-swap`). Several changes during a running render are merged into one notification. Keepalive comment every 25 s. Works only within one server process; several instances would need a bus (e.g. NATS).

## Game state

Only the game (mode, settings, players in throwing order) and the thrown darts are stored.
The current state is always computed by replaying the darts (`game.Game.Play`, one `Play` function per mode).
New game modes follow the same pattern; game state is not additionally stored in the database. What all modes share (who's up, round, legs, bull-off, waiting for the takeout, winner, throwers, best leg) lives in the turn flow `turn`: a mode calls `Advance`/`End`/`Finish` per dart, resets its points in `OnLegStart` and embeds `turn.State` (`t.State(playerIDs)`) into its state at the end. Service and interface read these shared fields via `game.Common(state)` and `GameView.Common()` without distinguishing by mode. After that a new mode only needs: settings and `PlayLog` in its own package, one case each in `game.Game.Validate`/`Play`, `stats.Summary.Add`, in the start form (`home.templ`, `controller.gameFromForm`) and in `game.templ` (player card, history, statistics).

**Legs and end of game** (all modes, `pkg/domain/game/turn`; user's decision 2026-10-11, one single end rule): whoever checks out wins the leg immediately – no catching up, no draws (previously the remaining players of the round were allowed to catch up; that was deliberately dropped). A game goes over `game.Game.Legs` (first to N, 1–`game.MaxLegs`, default 1); whoever has won that many legs first wins (`winner_id`). Each leg is started by the next player (leg k: player (k−1) mod n), a round is over when the leg's starter is up again. `turn.Log.Turn` produces the flow, `Turn.OnLegStart` resets the mode's points. On a board a won leg also waits for the takeout (`LegOver`) before the next one begins; only the end of the game doesn't wait.

**Sets** (user's wish 2026-10-14): optional `game.Game.Sets` (first to N sets, 1–`game.MaxSets` = 7, default 1 = no sets); with sets `Legs` is the legs to win a set. Whoever first wins `Legs` legs wins the set, whoever first wins `Sets` sets the game. Starters: each set is started by the next player after the previous set's starter (the bull-off winner starts set 1), within a set the legs go round from the set's starter – as in tournaments, so with two players the throw alternates between sets too. `turn.Turn` counts `Set`, `Leg` (in the set), `SetLegs` (legs won in the current set), `SetsWon` and `LegsWon` (whole game, for the statistics); the state has `set`, `set_over` (waiting for the takeout after a won set) and `set_winner`; the modes' players `sets` and `set_legs`, their turns `set`. Game page: "Set N" badge, sets as sky squares before the leg dots (legs of the current set), "Set an …!" banner, history headed "Set N · Leg M". Statistics don't count sets yet. Breaking: the table `games` has a new column `sets`, so an existing database must be deleted. Sets won are in the statistics (`stats` `Sets`).

**Bull-off** (option `game.Game.BullOff`, user's wish 2026-10-11: automatic via camera): before leg 1 every player throws one dart, the darts are in the normal log, `turn.Log.Replay` doesn't pass them to the mode (`turn/bulloff.go`). Closest to the bull starts – distance from the camera position, without position bullseye beats bull beats everything else; on a tie only the tied players throw again (`Round`) – or the players decide who was closer (especially without camera, e.g. both in the bull): `service.DecideBullOff` stores `games.bull_off_closer`, when replaying this replaces the throw-off at the first tie this player is part of (`turn.Log.BullOffCloser`). Only possible while there currently is a tie (`bull_off_tie` in the game state). A takeout during the bull-off changes nothing. The modes report `bulling_off` and `bull_off` (darts with player and round); the game page then shows buttons Bullseye/Bull/Daneben instead of the keypad, the throws with distance and "… beginnt!". `turn.Turn.Throwers` records for all entries who threw.

Statistics per game over all legs (X01 sums the points in `Stats.Points` for the average), best leg instead of best game (`BestLeg`), won legs per player (`Legs`, over the whole game also with sets). The game page shows legs as dots in the player card, "Leg N" in the title, "Leg an …!" after a leg win and separates the history by legs.

**X01** knows the out modes `single`, `double` and `master` (double or triple). Checkout suggestion (`x01.OutMode.Checkout`): with double out the usual route of the checkout tables (user's wish, 2026-10-11): 170–41 from the table by Guido Wessendorf (University of Münster, `checkout_table.go`; there is no single official pro table, other tables differ e.g. at 90/99/101), below that even numbers straight at the double, odd ones set up with a single on 32/16/8/4/2. If the darts of the visit aren't enough for that route (e.g. 101 with 2 darts), and with single/master out, it searches all routes with the remaining darts (~2 ms) and takes the one with the fewest darts, then the best target field (D20, D16, D8, … bull last; with single out preferably a single), then the easiest setup darts. `TestCheckoutValid` plays every suggestion for 1–180, so it also checks every table entry. The game page shows it in the empty dart tiles (`checkoutSuggestion`), not while waiting for the takeout.

**Waiting for the takeout:** games on a board (`game.Game.WaitsForTakeout`) wait after every visit (also after a bust) for the darts to be pulled before the next player is up; the last throw that ends the game doesn't wait. The takeout is an entry in the same log as the darts (`throw.Dart.Takeout`, table `darts`, order = ID) and is taken into account when replaying (`turn.Log.Replay`). Undo simply deletes the last entry. The camera reports it (`POST /api/board/takeout`), otherwise the players confirm it on the game page ("Pfeile gezogen", `POST /games/{id}/takeout` or `/api/games/{id}/takeout`). A dart while waiting is an error when replaying (`turn.ErrWaitingForTakeout`), so new darts are rejected. `turn.End`/`Finish` count the dart that ends the visit (bust, checkout), so the waiting visit shows the correct number of darts. Games without a board switch after three darts as before.

**Correcting darts** (`service.EditDart`; in the interface click any dart in the history or the running visit): the entry is changed and everything is replayed. Rejected if the log no longer works out afterwards (replay error, e.g. game would be over before later darts) or if it would change who threw later darts (`ErrChangesThrowers`, e.g. bust in the middle of the visit) – then undo instead. For this the turn flow provides the thrower per log entry (`turn.State.Throwers`).

**Aborting** (`service.AbortGame`, host only, also in the lobby): sets `games.aborted_at`; the game is kept including its darts but accepts nothing anymore (`ErrAborted`, 409), `CanThrow` is then false, open invitations disappear from `Invitations`, the camera skips it (`activeGame`) and statistics don't count it (it is never `GameOver`). Finished games can't be aborted.

**History:** both modes provide the history of all visits (X01: `x01.State.Turns` with darts, points, remaining, bust, checkout; Around the Clock: `aroundtheclock.State.Turns` with darts, hit per dart, target before/after); the last one can still be running, the game page shows it newest first. A visit is complete as soon as it is over – even if it is still waiting for the takeout (`t.Waiting()`); previously X01 visits on a board therefore didn't count for highest visit/100+/140+/180.

## Tournaments

User's wish 2026-10-14: every game mode as a tournament, with friends, statistics and a page per tournament.

- **Formats** (`pkg/domain/tournament`): knockout (`Knockout`) and all against all (`RoundRobin`), 3–32 players (`MinPlayers`/`MaxPlayers`; 32 since 2026-10-14, before 16; with 32 the knockout has five rounds from the Sechzehntelfinale, all against all 31 rounds with 496 games). Groups + knockout was discussed: it would reuse table and bracket, but needs group assignment, "best N per group go through", seeding the bracket from the groups and the stage transition – not built yet, see [TODO](TODO.md).
- **Stored, like a game, only what is needed to compute it:** `tournament.Tournament` with `Format`, `Settings` (a `game.Game` as template: mode, legs, sets, bull-off, board, mode settings; JSON column) and `PlayerIDs` in the order of the draw (shuffled in `service.CreateTournament`; the seeds of the bracket). Its games are ordinary games with `games.tournament_id` and `tournament_match` (index in the schedule), created by `Tournament.Game`. `tournament.Plan` computes schedule, bracket or table and winner from the results of the games (`service.tournamentView`: per match the newest game that wasn't aborted; an aborted game doesn't decide the match, it can be started again). Nothing about the schedule is stored, so it can't get out of sync.
- **Knockout:** bracket of the next power of two, seeds placed so that 1 and 2 meet only in the final (`seeding`: 1 8 4 5 2 7 3 6), the missing players are byes (`Match.Bye`) that go to the best seeds and advance without a game. A later match gets its players once the earlier ones are decided.
- **All against all:** circle method (`roundRobin`), every player once per round, with an odd number one sits out; the fixed player alternates who throws first. Table: wins, then difference of sets (legs without sets) won and lost, then more won, then the draw (`Standing`).
- **Players and invitations:** created from the normal start form ("Spielform": single game, knockout or all against all; `controller.createGame`), so mode and settings are chosen as for a game. Own player, guests and invited friends (`service.invitedPlayers`, shared with `CreateGame`); friends accept once for the tournament (`tournament_invitations`, `tournament.Invitation`; declining removes the player). Strangers aren't possible yet: that would only need the friend check in `invitedPlayers` relaxed plus a way to find/join tournaments (TODO, together with games).
- **Playing:** matches are played one after another with the same settings; the tournament page offers "Spiel starten" for every match whose players are known (`service.StartMatch`). The owner may start all, a player with an account only their own matches (`TournamentView.CanStart`). The game's owner is the tournament's owner; its players with an account get accepted invitations (`store.CreateGame` with `InvitationAccepted`), so they don't accept every game. The game page links back ("Zum Turnier"). The tournament page isn't updated live (reload); the games are.
- **Page** (`view/tournament.templ`): bracket in columns per round (Achtelfinale … Finale) or table plus matches by round, each match with its score (sets, legs without sets: `game.Game.Score`) and start button or link to its game, winner banner, statistics of the players over the finished games of the tournament (`TournamentView.Stats`, `stats.Summary.Add` per game), abort for the owner. The start page lists the last tournaments and the tournament invitations.
- **Abort:** only the owner, only while not over; an aborted tournament doesn't count in the statistics, its finished games do (like any game).
- No JSON API for tournaments yet (TODO).

## Statistics

Statistics are computed like the game state by replaying the darts and are not stored:

- Per game every mode attaches them to its player state (`x01.PlayerState.Stats`, `aroundtheclock.PlayerState.Stats`), so they are in every `state` of the API.
- X01: 3-dart average = points scored × 3 / darts thrown (bust visits count 0 points, their darts count); visits count for highest visit and 100+/140+/180 only once they are over.
- `pkg/domain/stats` sums over all finished games (running ones don't count) for the own player and the guests of an account (wins, won legs and sets, best leg), and per mode the finished tournaments taken part in and won (`Summary.AddTournament`; aborted ones don't count, their games do like every game).
- **Positions** (user's decision, 2026-10-11, for later statistics and heatmap): camera darts store the fused position in mm (`throw.Dart.X`/`Y`, columns `x`/`y` in `darts`; origin at the bull, Y towards the 20), darts entered by hand have none (`AddDart` discards them, only `AddBoardDart` sets them). No values per camera and nothing from the calibration – that stays with the client. A correction (`EditDart`) only changes the field, the position stays: darts whose field isn't `dartboard.Score(x, y)` were corrected. Positions are in board millimetres and comparable across camera setups. darts-o-cam has to send correct positions itself; the server stores them for statistics and doesn't correct anything (user's decision, 2026-10-13), see [darts-o-cam → known limitations](../darts-o-cam/AGENTS.md#known-limitations).
- The overall statistics replay all games of the account. If that gets too slow with very many games, a cache per finished game is the obvious next step.

## API

All endpoints under `/api` except `register`/`login`/`logout` need the session cookie from `POST /api/login`.

- `POST /api/register` (`{"username", "name", "password"}`), `POST /api/login` (`{"username", "password"}`), `POST /api/logout`
- `GET /api/players` (own player + guests), `POST /api/players` (`{"name"}`, creates a guest)
- `GET /api/boards` (with `last_used_at` once a game was started on the board), `POST /api/boards` (`{"name"}`), `DELETE /api/boards/{id}`
- `GET /api/games`, `POST /api/games`, e.g. `{"mode": "x01", "legs": 3, "player_ids": [1, 2], "invite": ["ben"], "board_id": 1, "x01": {"start_score": 501, "out_mode": "double"}}`
  or `{"mode": "around_the_clock", "player_ids": [1], "around_the_clock": {"bull": true, "multiplier_skips": false}}`; `player_ids` only own player and own guests; `invite` only friends, they are added at the end of the throwing order
- `GET /api/games/{id}`: game, players, invitations, log (`darts`, including entries with `takeout: true`), computed state as well as `ready`, `is_owner`, `can_throw`, `invited`
- `POST /api/games/{id}/darts`: `{"number": 20, "multiplier": 3}` or `{"miss": true}`; bull is `25`
- `POST /api/games/{id}/takeout`: darts were pulled (only relevant for games on a board)
- `POST /api/games/{id}/bull-off`: `{"player_id": 2}` – on a bull-off tie this player was closer
- `POST /api/games/{id}/abort`: abort the game (host only), afterwards `aborted_at` is set in the game
- `DELETE /api/games/{id}/darts/last`: undo the last dart
- `PUT /api/games/{id}/darts/{dartID}`: correct a dart (body as when entering)
- `POST /api/boards/{id}/token`: new camera token for an own board (replaces the old one), `GET /api/boards` shows `has_token`
- For cameras, with `Authorization: Bearer <board token>` instead of a session: `POST /api/board/darts` (dart as above plus optional `"x"`, `"y"`: position in mm, both or neither; goes to the board's running game), `POST /api/board/takeout`
- `GET /api/stats`: overall statistics of the own player and the guests
- `GET /api/friends` (friends, incoming and sent requests), `POST /api/friends` (`{"username"}`, accepts an already received request), `POST /api/friends/{accountID}/accept`, `DELETE /api/friends/{accountID}` (remove, decline or withdraw)
- `GET /api/invitations`, `POST /api/games/{id}/accept`, `POST /api/games/{id}/decline`, `DELETE /api/games/{id}/invitations/{playerID}` (host uninvites)

## Testing

- Rules and service (accounts, permissions, invitations against in-memory SQLite): `go test ./pkg/...`
- Web interface: with [moli](https://github.com/lexmount/moli) (user's decision, 2026-10-11; previously Lightpanda, moli can additionally take screenshots and waits for promises in `--eval`). Procedure:
  - Log in with curl (`curl -c cj -X POST …/api/login -d '{"username":…,"password":…}'`) and pass the cookie file to moli: `moli fetch <url> -c cj`. **Not** `-H "Cookie: …"`: that only applies to the page itself, htmx requests and the SSE connection then run without a session.
  - Clicks and checks: `--eval "(async () => { document.querySelector(…).click(); await new Promise(r => setTimeout(r, 800)); return document.querySelector('#board').textContent })()"` – the return value ends up on stdout. Structure: `--dump semantic_tree_text`.
  - Screenshots: `--layout --dump screenshot` (or `screenshot_full`) > PNG. Not pixel-exact like Chrome (hidden radio buttons in `btn-check` shine through, "↶" is missing in the font); use a real browser for details. No option for the window size (always 1920×1080), so the phone view can't be checked with it. moli always shows `<details>` expanded. moli shifts SVG text with a negative `viewBox` origin incorrectly, therefore the board SVG draws with origin 0.
