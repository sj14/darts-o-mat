# darts-o-mat

## About these documents

The `AGENTS.md` files describe the project as it is meant to be now: what exists, the rules, and the reason where it isn't obvious.

- **Everything here was decided with the user.** Don't deviate from a rule or a design described here on your own; ask first.
- **No history.** When and how something came about, what it replaced, who wished for it: that is in git. No dates, no "user's decision", no "before, …" in these files. Measurements keep their numbers and the events or test data they were taken on, not their date.
- **What was considered and dropped** stays, with its reason, in a "Rejected" section of the document it concerns, so it isn't proposed again.
- **What isn't verified** (not run on Windows, not installed on a real Linux, …) is said plainly; it is part of the current state.
- **Open work** and what was set aside goes into the `TODO.md` files, with date and reason (see [documentation](#documentation)).

## Goal

The goal of the project is a free, open-source darts scorer that you can host yourself, with automatic detection of the darts by cameras at the board.

The project consists of two programs:

- **Server** [`cmd/darts-o-mat`](internal/server/AGENTS.md): web interface and API, accounts, games, game state, statistics.
- **Camera** [`cmd/darts-o-cam`](internal/cam/AGENTS.md): detects darts with cameras at the board and reports them to the server; calibration, image processing.
  Camera access per operating system lives in [`internal/cam/camera`](internal/cam/camera/AGENTS.md).

## Audience

darts-o-mat is for people who know little about IT as well as for those who do. That holds for the whole project and above all for **darts-o-cam**: its downloads, instructions, the project page and its interface have to work for someone who has never used a terminal: a download link per system with plain labels (no jargon like "amd64"; "Intel/AMD", "ARM", "Apple silicon" plus a hint which one to choose), steps in the right order, no command lines for the camera.

For the **server** compromises are fine, since not everyone hosts their own server: commands, containers and Kubernetes are fine in its README, just not on the first page of the project (link to the server README instead). Exception: the project page shows the server's one-line install (`install.sh --server`) and the `systemctl` line of the Linux packages next to the download links.

When writing or changing the project page, the READMEs or texts of the camera's interface, ask: could someone without IT knowledge do this?

## From the home server to SaaS

darts-o-mat has to work both as a small self-hosted program (one person, one binary, a SQLite file, no accounts anywhere else) and as a large service with many strangers on one installation. Rules that follow:

- **The small installation needs nothing extra.** Whatever only a big or public installation needs (protection against bots such as a captcha, later perhaps rate limits, mail, an external database) is **optional and off by default**: it is switched on by a flag or environment variable, and a server without it behaves exactly as without the feature, with no network call to a third party and no extra field in a form. No feature may make the small installation depend on a service or a key.
- **Exception: cheap protection for everybody is on by default** (the limit on failed logins: no third party, no key, no extra field); the operator can turn it off or change it with a flag.
- **Several replicas of the server** are possible with PostgreSQL, for the big installation: `LISTEN/NOTIFY` carries the live updates, an advisory lock orders the changes, and everything else lives in the database, see [server → replicas](internal/server/AGENTS.md#replicas). New state must work across replicas as well: a notification through the bus, or a row in the database. Memory is only for a cache whose differences between replicas do no harm. The small installation stays one process without a bus or anything else extra.
- **Exception: the hint about a newer version** (`internal/common/update`, in the footers of the server and of darts-o-cam) asks GitHub for the latest release when the program starts and then once a day. A failed check shows nothing; a build that isn't a release (a git revision as its version) never shows a hint.
- **Keys and secrets** come as a flag **and** as an environment variable (`DARTS_O_MAT_…`; the flag's default is the variable), because flags show in the process list and the container and Helm chart work with variables. A half-set configuration (one of two keys) stops the server at the start instead of running unprotected.
- **Third parties** are named in the documentation with what they get to see, and are loaded only on the pages that need them (never in the layout); the app's own files stay embedded, no CDN (see [technologies](#technologies)).
- When a feature for the big case would make the code for the small one more complicated, say so before building it (see [code quality](#code-quality)).

## Documentation

The documents are split by area and link to each other. The READMEs of the two programs are in their `cmd/` folders (they are the first thing someone looking at a program finds, and the release archives ship the server's), the `AGENTS.md` and `TODO.md` in `internal/server` and `internal/cam`:

| Area | For humans | For agents | Open tasks |
| --- | --- | --- | --- |
| Whole project | [`README.md`](README.md) | this file | [`TODO.md`](TODO.md) |
| Server | [`cmd/darts-o-mat/README.md`](cmd/darts-o-mat/README.md) | [`internal/server/AGENTS.md`](internal/server/AGENTS.md) | [`internal/server/TODO.md`](internal/server/TODO.md) |
| Camera | [`cmd/darts-o-cam/README.md`](cmd/darts-o-cam/README.md) | [`internal/cam/AGENTS.md`](internal/cam/AGENTS.md) | [`internal/cam/TODO.md`](internal/cam/TODO.md) |
| Camera access | [`internal/cam/camera/README.md`](internal/cam/camera/README.md) | [`internal/cam/camera/AGENTS.md`](internal/cam/camera/AGENTS.md) | [`internal/cam/camera/TODO.md`](internal/cam/camera/TODO.md) |

- All documentation is written in English. The user interfaces are translated (English, German), see [languages](#languages).
- Whatever concerns the whole project (goal, rules, shared stack, structure, how server and camera work together) goes into the documents in the root folder. Whatever concerns only the server or only the camera goes into its folder; details of camera access per operating system go into `internal/cam/camera`. Don't maintain anything twice, link instead.
- `AGENTS.md` files follow [the rules above](#about-these-documents).
- `README.md` files are for humans only: short, clear and understandable without prior knowledge (what is it, how do I start it, how do I try it). No implementation details, architecture or instructions for agents in the READMEs; those belong in the `AGENTS.md` files.
- **Screenshots** live in `site/screenshots` (English only) and are used by the README and the project page; redo them when the look of the pages changes noticeably. How to take them: [server → testing](internal/server/AGENTS.md#testing).
- When something changes that affects users (starting, game modes, usage), update the matching README as well.
- **Keep the `TODO.md` files up to date**: with every change, remove what is done, reword what is only partly done, and add what was left out or decided against for now (with the date and the reason). Check them before you report a task as finished.
- Open tasks live in a `TODO.md` per area, next to its `AGENTS.md`: a task of a single part goes into its folder, one that concerns several parts or the whole project into the root [`TODO.md`](TODO.md), which links to the others.

## Code quality

Simple, elegant, idiomatic code matters to the user.

- **Don't repeat yourself:** a rule or a check lives in one place, and every way in calls it. When the same condition, check or sequence of steps shows up a second time, move it into one function (in the domain if it is a rule of the game, in the service if it needs the store) instead of copying it; a copy drifts apart, and then one of the ways in behaves differently. Examples: `service.checkSeatBoard` is the one check of the board a player takes part with; the API maps errors to status codes in one place (`writeError`).

- If a change of the intended behaviour (not only of the code) would make an implementation considerably simpler, ask the user before building the complex version: name the simpler behaviour and what it gives up, and put it first as the recommended option. TODO entries describe a wish, not a fixed specification.
- Don't implement every feature blindly: if something brings little benefit but makes the code considerably more complicated or worse, first state the effort and the consequences for the code, propose a simpler alternative or advise against it.

## Languages

English is the default language, German the first translation; more languages may follow, also only partly translated.

- **Which language:** the account's choice (`account.Account.Language`; the page header shows the current language as a flag, choosing another stores it; there is no "like the browser" option, so once chosen it stays; `service.SetLanguage` with an empty language would clear it), else the browser's `Accept-Language` (only supported languages count, `de-AT` is German), else English (`i18n.Default`). `internal/common/i18n` negotiates it and carries it in the request context (`i18n.Middleware` for the browser, `auth.Require` for the account's choice).
- **Texts:** per interface an interface with one method per text (server: `view.Texts`, darts-o-cam: `camui.Texts`; `T(ctx)` in templates and handlers); parameters and plurals are plain Go. English (`textsEN`) implements every method, so a missing English text doesn't compile. Every other language embeds the English type and overrides what is translated (`textsDE struct{ textsEN }`), so whatever isn't translated falls back to English instead of breaking. `TestTranslations` (in both packages, with `i18n.Empty` and `i18n.Untranslated`) calls every text in every language: a text that comes out the same as in English isn't translated, except those listed in `sameInAllLanguages` (Bull, Checkout, …); languages in `complete` (German) fail the test then, others only log what's missing.
- **New texts:** never write German or English directly into a template or a handler; add a method to `Texts`, implement it in English and German.
- darts-o-cam has no accounts: its language is only the browser's (`i18n.Middleware` in `camui`); in its own window (glaze) that's the system's web view. Its error texts from `camapp`/`vision` (e.g. in "Detection stopped: …") stay English.
- Errors meant for the user are sentinel errors or types in the code and get their text in the handler (e.g. `camui.calibrationError`), not formatted strings.
- Numbers are formatted by the language too (server `Texts.Decimal`/`Percent`, darts-o-cam `Texts.Number`: 90.0 / 90,0). The dart labels of the interfaces come from `dartLabel` in each package ("Miss"/"Daneben"); `dartboard.Hit.String` stays for logs and `darts-o-cam analyze`. Texts of JavaScript come from data attributes of the page (sound toggle).

## Breaking changes

The project isn't in use yet: breaking changes (API, calibration format) are allowed if they make the code more idiomatic or simpler. Don't build compatibility workarounds for old clients or calibration files; after such a change, explicitly tell the user at the end what they have to adjust or regenerate.
The database keeps its data: every change of the schema is a migration for both databases, see [server → database](internal/server/AGENTS.md#database).

## Technologies

New features build on the existing stack; don't introduce additional frameworks or libraries without asking.

- **Language:** Go (version see `go.mod`)
- **HTTP routing:** [chi](https://github.com/go-chi/chi) v5
- **Frontend:** server-side rendered HTML templates with [templ](https://templ.guide) (included as a Go tool in `go.mod`; generate with `go tool templ generate`, or `make generate` for the CSS as well) and [htmx](https://htmx.org) for interactivity: handlers return templ fragments that htmx swaps into the page, instead of custom JavaScript or a JS framework. Applies to the server's web interface and to darts-o-cam's calibration interface (`internal/cam/camui`).
- **Design:** [Tailwind CSS](https://tailwindcss.com) v4 with a custom dark "pub" look: board colors red/green/cream as accents, font Barlow Condensed for headings. Numbers and dart labels of the game page (dart tiles, keypad, scores, the board's numbers) use Roboto Condensed (`font-number`): in Barlow Condensed M looks like 11 and 1 like T, and a fixed font rather than the system's keeps labels the same width on every device. Labels in tiles and buttons shrink to fit (`fitText`: the `text-fit` utility sizes the font from the container's width, `@container`, and the label's length, `--fit-len`, up to a largest size), so DANEBEN, TREFFER or BULLSEYE fit in any tile, language and screen width. Applies to all pages of darts-o-mat and to darts-o-cam's calibration interface (`internal/cam/camui`, same CSS file). Building blocks: `view/ui.templ` (`panel`, `pageTitle`, `sectionTitle`, `errorAlert`, `notice`, `choice`, `checkbox`, `statusBadge`) and class constants in `view/ui.go` and `camui/styles.go` (`btnPrimary`, `inputClass`, …). The game page is described in the [server document](internal/server/AGENTS.md#game-page).
  Source: `internal/server/frontend/view/tailwind.css` (`@theme` with colors `board-*`/`pub-*`, fonts `font-display` and `font-number`, animations `animate-dart-in`/`shake`/`celebrate`/`glow`/`target`; scans `view` and `camui`). `internal/common/static/tailwind.css` is generated with `go generate ./common/static` – after every change to classes in `.templ` or `.go` files of these packages. The file is committed so that `go run` works without Tailwind and without internet. `go generate` runs `tools/tailwind`, which downloads the Tailwind standalone CLI in a fixed version with checksum once into the user cache (no Node/npm). Classes that Go code assembles must appear there as whole words, otherwise Tailwind doesn't find them.
  **Performance** (scrolling must stay smooth): no `backdrop-blur`, no `background-attachment: fixed` (the background shimmer is its own `position: fixed` layer, `body::before`), no `filter`/`drop-shadow` on large elements and no animations of `box-shadow` – `animate-glow` only fades the opacity of a shadow layer (`::after`) in and out; elements using it must not have `overflow-hidden`, otherwise the glow is clipped. Animate only `opacity`/`transform`.
- **Static files:** htmx (including the SSE extension), the generated Tailwind CSS, the fonts (Barlow Condensed and Roboto Condensed, Latin subset, each with its OFL license) and the favicon are committed in `internal/common/static` and served via `go:embed`, no CDN (darts-o-mat should work without internet). The favicon (`favicon.svg`, used by darts-o-mat and darts-o-cam) is the board logo of the page header with the theme colors as hex (concentric red/green/cream/black like the header's radial gradient; its percentages refer to the distance to the element's corner, i.e. √2 × the radius: red to 31 %, green to 57 %, cream to 88 % of the disc's radius, the ring around it 1/10 of the total radius); `apple-touch-icon.png` (180×180, for iOS home screens, which don't take SVG) is rendered from the same rings – change both together.
- **Database and ORM** (server only): PostgreSQL and SQLite with [Bun](https://bun.uptrace.dev), see [server](internal/server/AGENTS.md#database).
- **Camera/image processing** (darts-o-cam only): no external programs; so far pure Go without cgo, see [camera](internal/cam/AGENTS.md) and [`internal/cam/camera`](internal/cam/camera/AGENTS.md). The desktop window uses [glaze](https://github.com/crgimenes/glaze) (system web view via purego, no cgo), see [camera](internal/cam/AGENTS.md#app).
  **cgo:** avoid it where a pure Go way is reasonable, but the option stays open for darts-o-cam when it clearly brings something (e.g. ONNX Runtime, OpenCV on Linux, a native desktop window). When proposing something that needs cgo, always name what it costs: build tools per system (Xcode command line tools on macOS, a C compiler and the library's headers on Linux, MinGW on Windows), no simple cross-compiling, no `CGO_ENABLED=0` build, and libraries the binary then needs at runtime. darts-o-mat (the server) stays without cgo.
- **Messaging (optional):** [NATS](https://nats.io) may be added when needed, e.g. to transfer darts from the camera (`darts-o-cam`) to the server (`darts-o-mat`). Live updates across several server instances use PostgreSQL's `LISTEN/NOTIFY`; NATS or Redis may come later as an optional choice behind the same interface, see the [server TODO](internal/server/TODO.md#technical).

## Structure

One Go module for the project: the programs' `main` packages are in `cmd/` (`cmd/darts-o-mat`, `cmd/darts-o-cam`), everything else in `internal/` (nothing outside this repository may import it, which a `pkg/` would invite), split by who needs it: `internal/server`, `internal/cam` and `internal/common`. The only module besides it is `tools` (packaging with nFPM and winres and their ~40 modules, own `go.mod`, which `replace`s the project with `..`), so those stay out of the project's `go.mod`.

Because the compiler doesn't keep the programs apart, `internal/common/layout_test.go` does: `common` imports neither `server` nor `cam`, `server` not `cam`, `cam` not `server`. `common` is what both programs share and must stay small and free of what only one needs; what only the server knows (the game's log entry `throw.Dart` with its database tags) stays in `internal/server/domain/throw`, so the camera doesn't compile in the database library.

- [`common`](internal/common): what both programs share: `dartboard` (board geometry, `Segment` and `Hit`, the dart as the cameras report it), `i18n`, `notify`, `sse`, `buildinfo`, `update`, `static`
- [`server`](internal/server/AGENTS.md): the web server (main package in [`cmd/darts-o-mat`](cmd/darts-o-mat); `api`, `auth`, `captcha`, `service`, `store`, `domain`, `frontend`)
- [`cam`](internal/cam/AGENTS.md): the camera app (main package in [`cmd/darts-o-cam`](cmd/darts-o-cam); `camapp`, `camui`, `vision`, `camera`)
- [`tools`](tools): packaging and generators, see below

In detail:

- `internal/server/api`: JSON API under `/api` (server)
- `internal/server/service`: application logic including permission checks (server)
- `internal/server/auth`: session cookie and middleware (server)
- `internal/server/domain`: domain logic (board, players, darts, game modes X01 and Around the Clock, tournaments, the texts of the imprint and privacy pages, the admin's settings of the server); `internal/common/dartboard` (board geometry) is used by server and camera
- `internal/server/store`: persistence (server)
- `internal/common/update`: finds out whether a newer release exists (GitHub, once a day) for the footers of server and camera, see [the rule above](#from-the-home-server-to-saas) (server, camera)
- `internal/server/captcha`: the optional check that a registration comes from a person (Cloudflare Turnstile), see [server → captcha](internal/server/AGENTS.md#captcha) (server)
- `internal/common/i18n`: language of the web interfaces (negotiation, request context), see [languages](#languages)
- `internal/common/notify`: tells subscribers about changes (`Signal`, `Hub` by key); `internal/common/sse`: streams Server-Sent Events for htmx (`Stream`, with keepalive); both used by server and camera
- `internal/common/buildinfo`: the binary's version (git revision) and the project's URL, shown in the footers of both web interfaces
- `internal/server/frontend`: the server's web interface: controllers and templ views (`view`); the embedded static files are in `internal/common/static`, shared with `internal/cam/camui`
- [`internal/cam/camera`](internal/cam/camera/AGENTS.md): camera access per operating system (macOS, Linux, Windows)
- `internal/cam/vision`: camera geometry and dart detection in pure Go (darts-o-cam)
- `internal/cam/camapp`: darts-o-cam at runtime: configuration, cameras, detection (darts-o-cam)
- `internal/cam/camui`: web interface of `darts-o-cam` (darts-o-cam)
- `tools/tailwind`: downloads and runs the Tailwind CLI for `go generate`
- `tools/macapp`: builds darts-o-cam as a macOS app bundle, see [camera → app](internal/cam/AGENTS.md#app)
- `tools/linuxpkg`: builds the Linux packages (.deb, .rpm, .tar.gz) of darts-o-cam and of the server, same place
- `tools/winpkg`: builds darts-o-cam's Windows .zip (.exe with icon), same place
- `site`: the project page for GitHub Pages (static HTML and a CSS file in the look of the app, English and German in `site/de`; no generator); `make site` puts it with the app's fonts and icons into `dist/site`, `.github/workflows/pages.yml` publishes it. Texts in both languages by hand, keep them in line with the [README](README.md)
- `install.sh` (with `install_test.sh`): the one-line installer of darts-o-cam and, with `--server`, of the server, see [camera → installer](internal/cam/AGENTS.md#installer)
- `Makefile`: `make macapp`, `make linux`, `make windows`, `make apps` (all), `make site`, `make generate` (templ and Tailwind CSS), `make tidy` (`go mod tidy` in both modules), `make clean`; only for these builds, the project page and what spans the modules, everything else is the go command
- `deploy/helm/darts-o-mat`: Helm chart of the server, see [server → deployment](internal/server/AGENTS.md#deployment)
- `.github/workflows`: `ci.yml` (tests, chart lint), `release.yml` (a tag `vX.Y.Z` builds binaries, image, chart and the camera's packages, see [server → deployment](internal/server/AGENTS.md#deployment)), `pages.yml` (project page)
- `Containerfile`: image of the server, see [server → structure](internal/server/AGENTS.md#structure)
- `tools/internal/appicon`: the app icon for all of them, rendered from `favicon.svg`; `tools/internal/version`: the packages' version from git

What the server packages do in detail is under [server → structure](internal/server/AGENTS.md#structure), the camera packages under [camera → structure](internal/cam/AGENTS.md#structure).

## How server and camera work together

- **The calibration stays with the client:** the calibration lives on the computer with the cameras (darts-o-cam's configuration), never on the server – that way the same board works with different servers; server and token are set separately in darts-o-cam's settings.
- **Connection:** one camera token per board, created on the server. darts-o-cam (token and server URL entered in its settings) uses it to send darts (with their position in mm) and the takeout to `POST /api/board/darts` and `/api/board/takeout`; the server enters them into the board's running game like darts typed in by hand. Server side: [server → camera connection](internal/server/AGENTS.md#camera-connection), client side: [camera → connection to the game](internal/cam/AGENTS.md#connection-to-the-game).
- **Positions** are only sent to the server as the fused position in mm (origin at the bull, Y towards the 20, `internal/common/dartboard`); no values per camera and nothing from the calibration. They are comparable across camera setups (heatmap). darts-o-cam has to send correct positions itself; the server stores them for statistics and doesn't correct anything, see [server → statistics](internal/server/AGENTS.md#statistics).

## Testing

- Everything without camera and browser: `go test ./...` (rules and service against in-memory SQLite, image processing against images in `internal/cam/vision/testdata`, camera access without a camera); `go test -short ./...` skips the few that take half a second or more (real camera images, the checkout table), CI runs them all.
- **Every test calls `t.Parallel()`**, except one that changes a package variable the package's other tests read (it says so in a comment); Go runs those first, before the parallel ones. A test that needs its own database opens a named in-memory SQLite and closes it (`t.Cleanup`, `Store.Close`), so `-count` starts fresh. Tests make password hashes with `service.Options.PasswordCost` set to `bcrypt.MinCost`: at the default cost every account and login costs about 85 ms.
- **On macOS** every new test binary waits about 0.7 s on its first start while the system checks it (Gatekeeper/XProtect), and `go test` builds a new one per package. Allowing the terminal under System Settings → Privacy & Security → Developer Tools skips that check for what it starts.
- Web interface in the browser: see [server → testing](internal/server/AGENTS.md#testing).
- Camera and detection on the real board: see [camera → debugging](internal/cam/AGENTS.md#debugging).

## Rejected

- **GoCV/OpenCV:** there are no OpenCV packages for Intel Macs with macOS 26 (Homebrew would build OpenCV with 26 dependencies itself for hours). Still an option where a package exists, see [camera → further options](internal/cam/AGENTS.md#further-options).
- **A Go module per program:** no `go test ./...`, needs `go.work` and `replace`, and the build differs in the container and in CI.
- **go-i18n or `golang.org/x/text/message`** with message files: missing texts only show at runtime, and their extraction doesn't know templ. Worth reconsidering once translators without Go come in.