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

- Support desktop and headless operating systems (e.g. a Raspberry Pi next to the board without a screen, controlled from another device's browser).
- Make it a simple app in desktop mode: start it once (no subcommand, no flags) and do everything in the web interface – choose the cameras, calibrate, connect to the server (server URL and token) and start/stop detecting – without restarting with different commands or flags.
- Store `calibration.json` in the user's config directory instead of the current folder, e.g. via `os.UserConfigDir()`: `~/Library/Application Support/darts-o-cam` on macOS, `~/.config/darts-o-cam` on Linux, `%AppData%\darts-o-cam` on Windows. Then it doesn't matter where darts-o-cam is started, and the reference/empty images next to it end up there too. `-calibration` stays to use another file.
- Use a new calibration for detection immediately, without restarting. This includes newly chosen cameras, which are currently only opened after a restart.
- Add cameras individually instead of as a batch: today "Kameras wählen" picks a device name and all cameras with that name are opened together (`vision.ListDeviceGroups`, `vision.OpenCamerasNamed`). Users should add (and remove) any number of single cameras, also of different models, e.g. two of one kind plus another one. Keep in mind that identical cameras can't be told apart by any hardware ID, only by their image (see [darts-o-cam → finding cameras](cmd/darts-o-cam/AGENTS.md#finding-cameras)): the UI has to show an image per camera to pick from – a single still image like in the calibration is enough, with a button to take a new one (e.g. to hold a hand in front of a camera to see which one it is), and the calibration keeps assigning them by reference image.
- Keep debug images automatically: save the camera frames (before/after, all cameras) of the last N detected darts to disk, like `detect -save` but as a ring buffer, so misdetections can be examined afterwards. Default e.g. 10, set with a flag (and in the web interface, see the desktop app item above); 0 saves nothing. Store them in a folder that survives restarts (not `/tmp`, which macOS clears) and not in the repo, since the images can show people. The web interface already shows the last 20 detections, but only in memory.
  - Maybe that is already enough: each of these 20 detections keeps the full before/after frames of all cameras in memory (`vision.Event.Before`/`After`). A button "save this detection" in the web interface would let users keep exactly the misdetections they notice, without writing every dart to disk. Decide between this and the ring buffer on disk before building either.
  - Note the memory this takes: 20 events × 3 cameras × 2 frames at 1280×720 (RGBA) are ~440 MB once the list is full – a lot for a Raspberry Pi. Keep fewer, or keep the frames compressed (JPEG) or downscaled.
- Fuse the tips with the dart axes (needs recordings first, see the debug images above: darts close to wires with known fields). Each camera's dart axis, mapped onto the board plane, passes through the real entry point no matter where the changed area ends, so it is immune to the tip overshooting along the axis (~2 mm inwards at ring wires); it is only exact across the axis. Intersecting the axes alone was tried and rejected (T4 only from 97.6 to 98.3 mm, one bad measurement pulled the bullseye 90 mm away: nearly parallel lines make the intersection unstable). Not tried yet: combine both in `FuseTips` as a weighted least-squares point – distance to the axis lines weighs strongly, the position along each axis (the detected tip) only weakly, so crossing axes remove the overshoot and nearly parallel ones fall back to about today's result; outlier cameras are dropped as today. Measure against the median on the recordings before keeping it.
- Make the detection work with other setups (other cameras, resolutions, distances, number of cameras), not only the development board's three Autodarts cameras at 1280×720:
  - Express the detection thresholds in board millimetres instead of pixels and convert them per camera with the calibration, which knows how many pixels a millimetre is at each spot. Today they are pixel values measured on the development board: blur 5×5, blobs < 15 px dropped, gaps up to 6 px merged, motion threshold 40 on every 4th pixel, camera assignment via 64×36 thumbnail with limit 25. Test first with own recordings in another resolution (e.g. 640×480); other camera models need recordings from such setups.
  - Check the remaining assumptions, e.g. the default device name "Autodarts DIY Cam" and how fusion behaves with 1, 2 or more than 3 cameras.
- Allow adjusting the camera settings (fps, resolution) and the detection thresholds manually in the web interface, with the computed values as defaults.
  - Optional, if the effort is reasonable: find good thresholds automatically. Needs ground truth: e.g. a guided routine ("stick a dart into T20, then D5, …") whose saved frames are re-evaluated with different thresholds (like `darts-o-cam analyze`), keeping the values that score the most darts correctly. Moderate effort (routine in the UI plus a parameter search); simpler first step: the manual setting plus the re-evaluation of saved events to compare values.
  - The same routine gives known positions to measure a systematic error of the tip detection (development board: ~2 mm inwards at ring wires, see [darts-o-cam → known limitations](cmd/darts-o-cam/AGENTS.md#known-limitations)) and correct it in darts-o-cam, so it sends correct positions; the server doesn't correct anything.

## Camera access ([pkg/camera](pkg/camera/README.md))

- Nothing open.
