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
- Option: let every player with an account leave a running game, not only the host abort it.
  - With 2 players, leaving aborts the game (as today).
  - With more than 2 players, the game continues without the player who left.
  - Note: the game state is computed by replaying the darts with a fixed player list, so leaving has to become an entry in the log (like the takeout), from which on the replay skips that player. Open questions: what happens to their darts and statistics, can the host also leave (who is host then), and can the host remove guests the same way.
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
- Use a new calibration for detection immediately, without restarting. This includes newly chosen cameras, which are currently only opened after a restart.
- Allow configuring the camera settings (fps, resolution) and detection settings (e.g. the motion check interval) in the web interface.

## Camera access ([pkg/camera](pkg/camera/README.md))

- Nothing open.
