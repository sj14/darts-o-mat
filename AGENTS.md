# darts-o-mat

## Goal

The goal of the project is an alternative to [Autodarts](https://autodarts.io) and [Scolia](https://scoliadarts.com).

The project consists of two programs:

- **Server** [`cmd/darts-o-mat`](cmd/darts-o-mat/AGENTS.md): web interface and API, accounts, games, game state, statistics.
- **Camera** [`cmd/darts-o-cam`](cmd/darts-o-cam/AGENTS.md): detects darts with cameras at the board and reports them to the server; calibration, image processing.
  Camera access per operating system lives in [`pkg/camera`](pkg/camera/AGENTS.md).

## Documentation

The documents are split by area and link to each other:

| Area | For humans | For agents |
| --- | --- | --- |
| Whole project | [`README.md`](README.md) | this file |
| Server | [`cmd/darts-o-mat/README.md`](cmd/darts-o-mat/README.md) | [`cmd/darts-o-mat/AGENTS.md`](cmd/darts-o-mat/AGENTS.md) |
| Camera | [`cmd/darts-o-cam/README.md`](cmd/darts-o-cam/README.md) | [`cmd/darts-o-cam/AGENTS.md`](cmd/darts-o-cam/AGENTS.md) |
| Camera access | [`pkg/camera/README.md`](pkg/camera/README.md) | [`pkg/camera/AGENTS.md`](pkg/camera/AGENTS.md) |

- All documentation is written in English (user's decision, 2026-10-12). The user interface stays German for now (translations are on the TODO list).
- Whatever concerns the whole project (goal, rules, shared stack, structure, how server and camera work together) goes into the documents in the root folder. Whatever concerns only the server or only the camera goes into its folder; details of camera access per operating system go into `pkg/camera`. Don't maintain anything twice, link instead.
- `README.md` files are for humans only: short, clear and understandable without prior knowledge (what is it, how do I start it, how do I try it).
  No implementation details, architecture or instructions for agents in the READMEs; those belong in the `AGENTS.md` files.
- When something changes that affects users (starting, game modes, usage), update the matching README as well.

## Code quality

Simple, elegant, idiomatic code matters to the user (noted 2026-10-11).
Don't implement every feature blindly: if something brings little benefit but makes the code considerably more complicated or worse, first state the effort and the consequences for the code, propose a simpler alternative or advise against it.

## Breaking changes

The project isn't in use yet: breaking changes (database schema, API, calibration format) are allowed if they make the code more idiomatic or simpler. Don't build migrations or compatibility workarounds for old data.
After every change that makes an existing database unusable, explicitly tell the user at the end to delete the old database (e.g. `darts-o-mat.db`). Likewise for other breaking changes (API, calibration file), say what they have to adjust or regenerate.

## Technologies

New features build on the existing stack; don't introduce additional frameworks or libraries without asking.

- **Language:** Go (version see `go.mod`)
- **HTTP routing:** [chi](https://github.com/go-chi/chi) v5
- **Frontend:** server-side rendered HTML templates with [templ](https://templ.guide) (included as a Go tool in `go.mod`; generate with `go tool templ generate`) and [htmx](https://htmx.org) for interactivity: handlers return templ fragments that htmx swaps into the page, instead of custom JavaScript or a JS framework. Applies to the server's web interface and to darts-o-cam's calibration interface (`pkg/camui`).
- **Design (user's decision 2026-10-11):** [Tailwind CSS](https://tailwindcss.com) v4 (previously Bootstrap 6 alpha) with a custom dark "pub" look: board colors red/green/cream as accents, font Barlow Condensed for numbers and headings. Applies to all pages of darts-o-mat and to darts-o-cam's calibration interface (`pkg/camui`, same CSS file). Building blocks: `view/ui.templ` (`panel`, `pageTitle`, `sectionTitle`, `errorAlert`, `notice`, `choice`, `checkbox`, `statusBadge`) and class constants in `view/ui.go` and `camui/styles.go` (`btnPrimary`, `inputClass`, …). The game page is described in the [server document](cmd/darts-o-mat/AGENTS.md#game-page).
  Source: `pkg/frontend/view/tailwind.css` (`@theme` with colors `board-*`/`pub-*`, font `font-display`, animations `animate-dart-in`/`shake`/`celebrate`/`glow`/`target`; scans `view` and `camui`). `pkg/frontend/static/tailwind.css` is generated with `go generate ./pkg/frontend/static` – after every change to classes in `.templ` or `.go` files of these packages. The file is committed so that `go run` works without Tailwind and without internet. `go generate` runs `tools/tailwind`, which downloads the Tailwind standalone CLI in a fixed version with checksum once into the user cache (no Node/npm). Classes that Go code assembles must appear there as whole words, otherwise Tailwind doesn't find them.
  **Performance** (user noticed janky scrolling, 2026-10-11): no `backdrop-blur`, no `background-attachment: fixed` (the background shimmer is its own `position: fixed` layer, `body::before`), no `filter`/`drop-shadow` on large elements and no animations of `box-shadow` – `animate-glow` only fades the opacity of a shadow layer (`::after`) in and out; elements using it must not have `overflow-hidden`, otherwise the glow is clipped. Animate only `opacity`/`transform`.
- **Static files:** htmx (including the SSE extension), the generated Tailwind CSS and the font (with OFL license) are committed in `pkg/frontend/static` and served via `go:embed`, no CDN (darts-o-mat should work without internet).
- **Database and ORM** (server only): PostgreSQL and SQLite with [Bun](https://bun.uptrace.dev), see [server](cmd/darts-o-mat/AGENTS.md#database).
- **Camera/image processing** (darts-o-cam only): pure Go without cgo and without external programs, see [camera](cmd/darts-o-cam/AGENTS.md) and [`pkg/camera`](pkg/camera/AGENTS.md). GoCV was planned, but there are no OpenCV packages for Intel Macs with macOS 26 (Homebrew would have to build OpenCV with 26 dependencies itself for hours); therefore deliberately rejected on 2026-10-11.
- **Messaging (optional):** [NATS](https://nats.io) may be added when needed, e.g. to transfer darts from the camera (`darts-o-cam`) to the server (`darts-o-mat`) or to distribute live updates across several server instances.

## Structure

- [`cmd/darts-o-mat`](cmd/darts-o-mat/AGENTS.md): web server
- [`cmd/darts-o-cam`](cmd/darts-o-cam/AGENTS.md): camera tool (detect darts, calibrate, debug)
- `pkg/api`: JSON API under `/api` (server)
- `pkg/service`: application logic including permission checks (server)
- `pkg/auth`: session cookie and middleware (server)
- `pkg/domain`: domain logic (board, players, darts, game modes X01 and Around the Clock); `pkg/domain/dartboard` (board geometry) is used by server and camera
- `pkg/repos/store`: persistence (server)
- `pkg/frontend`: the server's web interface: controllers, templ views (`view`) and embedded static files (`static`, also used by `pkg/camui`)
- [`pkg/camera`](pkg/camera/AGENTS.md): camera access per operating system (macOS, Linux, Windows)
- `pkg/vision`: camera geometry and dart detection in pure Go (darts-o-cam)
- `pkg/camui`: local interface of `darts-o-cam` (darts-o-cam)
- `tools/tailwind`: downloads and runs the Tailwind CLI for `go generate`

What the server packages do in detail is under [server → structure](cmd/darts-o-mat/AGENTS.md#structure), the camera packages under [camera → structure](cmd/darts-o-cam/AGENTS.md#structure).

## How server and camera work together

- **The calibration stays with the client** (user's decision, 2026-10-11): the calibration file lives on the computer with the cameras, never on the server – that way the same board works with different servers; server and token are passed separately via `-server`/`-token`.
- **Connection:** one camera token per board, created on the server. `darts-o-cam detect` uses it to send darts (with their position in mm) and the takeout to `POST /api/board/darts` and `/api/board/takeout`; the server enters them into the board's running game like darts typed in by hand. Server side: [server → camera connection](cmd/darts-o-mat/AGENTS.md#camera-connection), client side: [camera → connection to the game](cmd/darts-o-cam/AGENTS.md#connection-to-the-game).
- **Positions** are only sent to the server as the fused position in mm (origin at the bull, Y towards the 20, `pkg/domain/dartboard`); no values per camera and nothing from the calibration. Stored positions plus corrections in the game are also the measurement series for the detection, see [server → statistics](cmd/darts-o-mat/AGENTS.md#statistics).

## Testing

- Everything without camera and browser: `go test ./...` (rules and service against in-memory SQLite, image processing against images in `pkg/vision/testdata`, camera access without a camera).
- Web interface with moli: see [server → testing](cmd/darts-o-mat/AGENTS.md#testing).
- Camera and detection on the real board: see [camera → debugging](cmd/darts-o-cam/AGENTS.md#debugging).
