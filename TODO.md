# TODO

Open tasks of the whole project. Tasks of a single part are in its own file:

- [Server](cmd/darts-o-mat/TODO.md)
- [Camera](cmd/darts-o-cam/TODO.md)
- [Camera access](pkg/camera/TODO.md)

## Whole project

- rate limits
- restructure packages
- Do not store json objects in the database (only when when it makes sense and agreed on upfront), create a new table instead
- Project page: add screenshots (game page, tournament, statistics, camera calibration; English and German, e.g. made with moli) and download links for darts-o-cam once there are GitHub Releases (a workflow that runs the `make` targets and attaches `dist/`). Then also enable Pages: Settings → Pages → Source: GitHub Actions.
