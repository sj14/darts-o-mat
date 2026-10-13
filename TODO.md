# TODO

## Whole project

- Translations: English user interface (currently German only).

## Server ([cmd/darts-o-mat](cmd/darts-o-mat/README.md))

- Unsure: switch the primary IDs to UUIDs.
- Allow others to watch games if they know the link.
- Tournament mode.
- Play with strangers (idea, not designed yet). For example:
  - When creating a game, tick something like "allow strangers"; the game waits in the lobby for them.
  - Finding a game: either a list of open games to join, or automatic matchmaking.
  - Today only friends can be invited, and games are hidden from everyone else (404); this would be the first exception.
- Statistics: add a heatmap of the dart positions.
- Confirm the end of a game: after the last dart the game is over but not yet completed; the players check it (e.g. correct a misdetected final dart from the camera) and confirm. Afterwards it can't be undone or corrected anymore.
  - Confirm automatically when one of the players starts a new game, maybe also after some time, so a forgotten confirmation doesn't leave games hanging.
  - Adds a state (running → over → completed) that undo, corrections, the camera and the "one running game per account" option have to respect (an unconfirmed game still counts as running).
  - Natural place for buttons like "Revanche" or "Neues Spiel".
- Show a statistics summary at the end: after a leg the statistics of that leg, after the game the statistics of the whole game (per player, e.g. average, highest visit, checkout, darts). Open: show the game summary as soon as the game is over (together with the confirmation, see above) or only once it is completed.
- Later, only once the statistics page gets measurably slow (builds on confirmed game ends): a statistics table per player and game mode with running totals, updated when a game is completed. Safe because completed games can't change anymore; sums just add up and highest/best values only need max/min. The replay stays for rebuilding the table when statistics are added or fixed. A cheaper first step: load the darts of all games in one query instead of one per game.
- Option: let every player with an account leave a running game, not only the host abort it.
  - With 2 players, leaving aborts the game (as today).
  - With more than 2 players, the game continues without the player who left.
  - Note: the game state is computed by replaying the darts with a fixed player list, so leaving has to become an entry in the log (like the takeout), from which on the replay skips that player. Open questions: what happens to their darts and statistics, can the host also leave (who is host then), and can the host remove guests the same way.
- Delete guests and delete the own account (undecided how). Always deleted: name, login, sessions, friendships, boards with camera tokens, the account's guests. Open:
  - Past games with the deleted player: keep them with the player shown as "Gelöschter Spieler" (players and their history stay intact; recommended) or delete them with all darts (fellow players, also friends, lose them and their statistics). Removing just the player from a game isn't possible: darts aren't linked to players, who threw is derived from the throwing order when replaying.
  - Guest in a running game: refuse ("finish or abort it first"; recommended) or abort the game automatically.
  - Account in running games (own ones and accepted ones of friends): abort them automatically and decline open invitations (recommended; refusing would block until friends finish, since only the host can abort) or refuse.
  - Deleting the account should ask for the password again.
- Boards (cameras):
  - During a game, allow switching from no board to a board; this also updates the board's "last used" time (`board.Board.LastUsedAt`, so far only set when a game is created on it).
  - A board belongs to at most one game at a time. If the selected board is already used by another game, show a warning but allow it; the board then moves to this game and the other game continues without a board. This replaces today's rule that camera darts go to the newest active game with the board.
- Option, undecided: allow only one running game per account (games it hosts or has accepted, including one waiting in the lobby).
  - For: matches reality (one person plays one game at a time), no forgotten games, and the board warning/moving above becomes unnecessary, because a board could then only be in its owner's single game.
  - Against: club or pub use – one account couldn't run guest-only games on two boards at the same time.
  - If done: starting a new game while one runs offers "continue it" or "abort it and start the new one" instead of just refusing; invitations stay open until the current game is finished or aborted.

## Camera ([cmd/darts-o-cam](cmd/darts-o-cam/README.md))

- macOS: a minimal `.app` bundle around the Go binary (folder with `Info.plist` and icon, no cgo): start by double click, and macOS asks for camera access for "darts-o-cam" with its own explanation (`NSCameraUsageDescription`) instead of for the terminal.
- Later (user's decision, 2026-10-13): a separate desktop app next to the headless one, e.g. with [Wails](https://github.com/wailsapp/wails) (check the state of v3 first). The desktop app has a native window, and closing it stops the cameras. The headless app stays as today. Both serve the same chi/templ/htmx handler.
  - One binary with a flag isn't possible: Wails links GTK/WebKitGTK on Linux via cgo, and the dynamic loader needs these libraries before `main` runs, so a headless Pi without them couldn't even start it. Loading them at runtime with purego would mean writing the window binding ourselves. macOS (WebKit always there) and Windows (WebView2 loaded at runtime) don't have this problem.
  - Rejected for this: opening Chrome/Edge in app mode (`--app=<url>`) or an installable web app (manifest). Both give a window without cgo, but closing it leaves darts-o-cam and the cameras running.
- Debug images: the frames of the last 20 detections are kept on disk (cache directory) and can be downloaded on the event page; `-save` keeps all. Maybe later: make the number configurable in the web interface.
- Fuse the tips with the dart axes (needs recordings first, see the debug images above: darts close to wires with known fields). Each camera's dart axis, mapped onto the board plane, passes through the real entry point no matter where the changed area ends, so it is immune to where the changed area ends along the axis (the series of 2026-10-13 found no common bias there, but single cameras are off by ±2–3 mm); it is only exact across the axis. Intersecting the axes alone was tried and rejected (T4 only from 97.6 to 98.3 mm, one bad measurement pulled the bullseye 90 mm away: nearly parallel lines make the intersection unstable). Not tried yet: combine both in `FuseTips` as a weighted least-squares point – distance to the axis lines weighs strongly, the position along each axis (the detected tip) only weakly, so crossing axes remove the overshoot and nearly parallel ones fall back to about today's result; outlier cameras are dropped as today. Measure against the median on the recordings before keeping it.
- Frame size per camera, from the modes the camera offers: today there is one wanted size for all cameras (settings), and each camera uses its own mode closest to it. With different camera models, each camera should list its real modes and get its own size. Needs a mode list in `pkg/camera` for all three systems (macOS: the device's formats, Linux: `VIDIOC_ENUM_FRAMESIZES`, Windows: the native media types, which `start` already walks through), a size per camera in the configuration and a choice on the camera's card instead of the settings. About 150–250 lines, mostly only testable with cameras on Linux and Windows; only worth it once different camera models are used.
- Check the detection with other setups (other cameras, resolutions, distances, number of cameras): the thresholds are in board millimetres now and scale per camera, but were only measured on the development board's three Autodarts cameras at 1280×720. Test first with own recordings in another resolution (e.g. 640×480); other camera models need recordings from such setups. Also check how fusion behaves with 1, 2 or more than 3 cameras.
- Find good detection thresholds automatically (optional, if the effort is reasonable; they can be set by hand under "Einstellungen"). Needs ground truth: e.g. a guided routine ("stick a dart into T20, then D5, …") whose saved frames are re-evaluated with different thresholds (like `darts-o-cam analyze`), keeping the values that score the most darts correctly. Moderate effort (routine in the UI plus a parameter search); simpler first step: re-evaluate saved events with different values to compare them.
  - A first series by hand (32 darts at wires, 2026-10-13) found no systematic error of the tip detection to correct, see [darts-o-cam → accuracy](cmd/darts-o-cam/AGENTS.md#accuracy); the routine would mainly help with the thresholds and with other setups.

## Camera access ([pkg/camera](pkg/camera/README.md))

- Nothing open.
