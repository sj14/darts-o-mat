# darts-o-mat (server)

The web app for playing darts: enter throws, the score is counted automatically. Part of [darts-o-mat](../../README.md); throws can also come from cameras via [darts-o-cam](../darts-o-cam/README.md).

## Game modes

- **X01** (e.g. 301 or 501): play down from the start score to exactly 0. To finish, depending on the setting any dart, a double, or a double/triple ("Master Out"). The game page suggests a checkout.
- **Around the Clock**: hit the numbers 1 to 20 in order, optionally the bull at the end. The next target lights up on the dartboard of the game page.

Whoever finishes first wins the leg immediately. When starting a game you choose how many legs are needed to win ("First to 3"); each leg is started by the next player. Optionally the legs are played in sets ("First to 3 sets of 3 legs"): whoever wins the legs of a set first wins the set, and whoever wins enough sets wins the game; each set is started by the next player. Who starts the first leg can be decided by a bull-off (option when starting a game): everyone throws one dart at the bull, the closest starts – with cameras to the millimeter. Optionally every turn has a time limit (30 seconds to 5 minutes): whoever takes longer misses the turn, and after three missed turns in a row they leave the game.

## Tournaments

Every game mode can also be played as a tournament: under "Format" on the start page choose a knockout tournament ("Knockout tournament") or all against all ("Round robin"), with 3 to 32 players – you, your guests and invited friends, brought together in its lobby like a game's. Starting it draws the order at random. On the tournament's page you start the games one after another; the bracket or the table fills in by itself, and below you see everyone's statistics in this tournament. Invited friends accept once for the whole tournament.

## Start

Download the server for your system from the [latest release](https://github.com/sj14/darts-o-mat/releases/latest) ([Linux](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_linux_amd64.tar.gz), [macOS](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_darwin_arm64.tar.gz) / [Intel](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_darwin_amd64.tar.gz), [Windows](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_windows_amd64.zip); arm64 versions are on the release page), unpack it and start it (in the Linux archive the program is `usr/bin/darts-o-mat`):

```sh
./darts-o-mat
```

On Linux, packages install it as a service ([.deb](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_linux_amd64.deb) for Debian, Ubuntu and Raspberry Pi OS, [.rpm](https://github.com/sj14/darts-o-mat/releases/latest/download/darts-o-mat_linux_amd64.rpm) for Fedora and openSUSE; arm64 versions are on the release page):

```sh
sudo apt install ./darts-o-mat_linux_amd64.deb
```

```sh
sudo systemctl enable --now darts-o-mat
```

(`sudo dnf install ./darts-o-mat_linux_amd64.rpm` for the .rpm.) The service listens on port 3000 and keeps the data in `/var/lib/darts-o-mat`; how to change the options is written at the top of `/usr/lib/systemd/system/darts-o-mat.service`. One line that picks the right file for your computer (Linux and macOS), checks it against the release's checksums and, run again later, updates it:

```sh
curl -fsSL https://raw.githubusercontent.com/sj14/darts-o-mat/main/install.sh | bash -s -- --server
```

(`--dry-run` only shows what it would do, `--version 1.2.3` takes another release. Without a package manager for .deb or .rpm, and on macOS, it puts the program into `~/.local/bin`.)

Or from the source, with [Go](https://go.dev/dl/) installed, from the project folder:

```sh
go run ./cmd/darts-o-mat
```

Then open <http://localhost:3000> in the browser. The interface follows your browser's language (English or German, otherwise English); change it with the flag at the top right once you're logged in. The data is stored in the file `darts-o-mat.db` in the current folder.

Options:

- `-addr :8080` – listen on another port
- `-max-login-failures 10` – lock a username for 15 minutes after that many failed logins, and a whole network after five times as many, as protection against guessing passwords (the default; `0` turns it off)
- `-client-ip-header X-Forwarded-For` – behind a proxy or ingress: the header it passes the visitor's address in, so the limit per network works (only if all requests come through that proxy)
- `-pprof localhost:6060` – for developers: serves Go's profiles (CPU, memory, goroutines) on that address, e.g. for `go tool pprof http://localhost:6060/debug/pprof/profile`. Off by default; keep it on `localhost`, there is no login
- `-turnstile-site-key …` and `-turnstile-secret-key …` – only for servers that strangers can register on: registering then needs a [Cloudflare Turnstile](https://developers.cloudflare.com/turnstile/) check ("I am not a robot"). Without both keys nothing is checked, and nothing from Cloudflare is loaded. The keys can also be set as `DARTS_O_MAT_TURNSTILE_SITE_KEY` and `DARTS_O_MAT_TURNSTILE_SECRET_KEY`.

Behind a proxy that provides HTTPS, the proxy has to send `X-Forwarded-Proto: https` (Caddy, Traefik and the nginx ingress do; plain nginx needs `proxy_set_header X-Forwarded-Proto $scheme;`), otherwise the login cookie isn't marked as HTTPS-only.

### In a container

Instead of Go, [Docker](https://www.docker.com) (or Podman) is enough. From the project folder:

```sh
docker build -f Containerfile -t darts-o-mat .
```

```sh
docker run -d -p 3000:3000 -v darts-o-mat:/data darts-o-mat
```

The data is kept in the volume `darts-o-mat`. Options go after the image name, e.g. `docker run … darts-o-mat -max-login-failures 5`.

The released image is `ghcr.io/sj14/darts-o-mat` (amd64 and arm64; `latest` is the newest stable release), so there is no need to build it: `docker run -d -p 3000:3000 -v darts-o-mat:/data ghcr.io/sj14/darts-o-mat`.

### On Kubernetes

With the [Helm](https://helm.sh) chart (one pod, the games in a volume):

```sh
helm install darts oci://ghcr.io/sj14/charts/darts-o-mat
```

Common settings, e.g. `helm install darts … --set ingress.enabled=true --set ingress.hosts[0].host=darts.example.com --set ingress.hosts[0].paths[0].path=/`. For PostgreSQL instead of the volume: `--set config.database=postgres --set postgresql.enabled=true` starts a simple PostgreSQL along with it; for a PostgreSQL server of your own instead `--set config.database=postgres --set config.postgres.dsn='postgres://…'` (or `config.postgres.existingSecret`). Only with PostgreSQL the server can run in several pods, e.g. `--set replicaCount=3`; an update then replaces them one by one without a break. All options with their comments: [`deploy/helm/darts-o-mat/values.yaml`](../../deploy/helm/darts-o-mat/values.yaml).

### With PostgreSQL instead of a file

```sh
docker run -d --name darts-postgres -e POSTGRES_PASSWORD=geheim -e POSTGRES_DB=darts-o-mat -p 5432:5432 postgres
```

```sh
go run ./cmd/darts-o-mat -db postgres -dsn "postgres://postgres:geheim@localhost:5432/darts-o-mat?sslmode=disable"
```

Instead of `-dsn` the environment variable `DARTS_O_MAT_DSN` works too, better for the password: options show up in the list of processes.

## Updates

The footer of every page says "Version … available" when there is a newer release. For that the server asks GitHub for the latest release when it starts and then once a day; GitHub sees the server's address and the version, nothing else. Nothing is updated by itself: download the new version from the [release page](https://github.com/sj14/darts-o-mat/releases/latest) (or run the install line above again, or pull the new container image) and start it; the database stays as it is.

## Playing

1. On your first visit create an account ("Register"). Every account is also your player. You may enter an email address; it is optional, nothing is sent to it yet, and you can change or remove it under "Settings". The first account is an **admin** of the server: under "Settings" → "Open administration" it closes the registration once everybody has an account (e.g. if darts-o-mat is publicly reachable), writes the imprint and the privacy policy, which every page links at the bottom (until they have a text, they say that there is none yet), and can make other accounts admins (by their username) or take the role away again. The last admin can't give it up.
2. Choose the game mode and its settings and create the game: it opens its **lobby**, with you in it. Players with their own account are added under **Freunde** by username; once they accept the request, you can **invite** them in the lobby, and they accept there. The friend list shows who is online or in a game (the lobby too, behind the name); "Appear offline" under "Settings" hides that about you.
3. In the lobby, add fellow players without an account as **guest players** (a name is enough), choose everyone's board, invite friends, and press **Start game** once everyone invited has answered. Who throws first is drawn then. You delete guests under "Settings". A player is in one game or tournament at a time: while someone is still in a running game, waits in a lobby or takes part in a tournament that isn't over, they can't start, join or accept another one (the message links to it); finish or leave it first. Your guests can play at the same time as you, e.g. on a second board: when you have guests, the start form asks who is in the lobby first.
4. Enter every throw: first choose single, double or triple, then tap the number – the keys then read e.g. "D20" or "T20". For the bull "Bull" (with "Double" it becomes "Bullseye"; there is no triple bull), for misses "Miss". Misses detected by the cameras show where they went, e.g. "M7" beside the 7. All players with an account can enter throws on their own device.
5. Mistyped or misdetected? Tap the dart in the running visit or in the history and choose what was really hit. "Undo last dart" undoes the last dart.
6. The game page plays sounds for hits, misses, busts and 180; toggle them at the top with "🔊 Sound on". Browsers only allow sounds once you have clicked or tapped the page. If the top says "🔈 Click for sound", click it – important when the cameras enter the throws and you don't touch anything else.
7. When a game is over, the last dart can still be corrected for a while, e.g. if the cameras got it wrong: until one of the players starts or joins another game or tournament, or for one minute after the last dart. In a tournament the winner goes on at once; starting the next match makes the earlier results final.
8. Had enough? Everybody can leave with "Leave game" (you leave with your guests): the others play on without you, and the game ends if fewer than two players would be left. In a game of one account alone that is "Abort game"; it stays in the list but doesn't count in the statistics. All players of a game are equal: nobody is the host, everybody enters the darts of their own players.
9. Under "Settings" you can change your email, delete your account (with your password; not as the only admin), and, in the player selection of a new game, guests with the ✕ next to them. Finished games stay for the others; the deleted player is shown there as "Deleted player".

During the game you see the average or hit rate and a small statistic for every player. The history shows every visit with its darts and how long each leg and set took; at the end you see how long the game (or the tournament) lasted. Under "Statistics" you find your values and those of your guest players over all finished games, and – for darts the camera saw – a heatmap of where your darts land on the board; the tabs above narrow both down to one game mode. A game counts once its result is final (see above). After a game you can give the other players a thumb up or down; others see the share of thumbs up in the list of open games (from three players on, each counts once). That list also shows how well each player plays the game's mode lately, so you find opponents of about your strength: the 3-dart average in X01, the hit rate in Around the Clock, over their last 20 finished games of it ("new" with fewer than 3). The filter "Strength: About mine" shows only the games of players about as strong as you, and "Quick match" finds an opponent by itself: you join a game that waits or wait for one yourself; the longer it takes, the bigger the difference may be (X01 501 double out or Around the Clock, one leg, a minute per turn). The statistics also show how often you left a running game or tournament yourself.

Under "Boards" you can create your dartboards (e.g. basement, club) and select them when starting a game; the board you used last is preselected. To have darts detected automatically, see [playing with cameras](../../README.md#playing-with-cameras).
