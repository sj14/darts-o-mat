# TODO

Open tasks of the whole project. Tasks of a single part are in its own file:

- [Server](server/TODO.md)
- [Camera](cam/TODO.md)
- [Camera access](cam/camera/TODO.md)

## Whole project

- Rate limits beyond failed logins (those are done, see [server → failed logins](server/AGENTS.md#failed-logins)): registering, creating games and tournaments, friend requests, the API in general.
- Check the new layout (folders `common`, `server`, `cam`, one Go module, `tools` apart; 2026-10-19) in CI, where it wasn't run yet: a release (`release.yml`) and the pages workflow. The container image, the packages and all tests were checked locally.
