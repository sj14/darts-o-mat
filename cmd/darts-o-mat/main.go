package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/sj14/darts-o-mat/internal/common/buildinfo"
	"github.com/sj14/darts-o-mat/internal/common/sse"
	"github.com/sj14/darts-o-mat/internal/common/update"
	"github.com/sj14/darts-o-mat/internal/server/api"
	"github.com/sj14/darts-o-mat/internal/server/captcha"
	"github.com/sj14/darts-o-mat/internal/server/frontend/controller"
	"github.com/sj14/darts-o-mat/internal/server/service"
	"github.com/sj14/darts-o-mat/internal/server/store"
)

func main() {
	var (
		dbType           = flag.String("db", "sqlite", "database to use: sqlite or postgres")
		dbDSN            = flag.String("dsn", "file:darts-o-mat.db", "data source name of the database")
		addr             = flag.String("addr", ":3000", "address to listen on")
		maxLoginFailures = flag.Int("max-login-failures", 10, "failed logins a username may have within 15 minutes before it is locked for a while (a network may have 5 times as many); 0 is no limit")
		clientIPHeader   = flag.String("client-ip-header", "", "behind a proxy: the header it passes the client's address in, e.g. X-Forwarded-For; only if all requests come through it")
		// The secret key is better passed in the environment: flags show up in the process list.
		turnstileSite   = flag.String("turnstile-site-key", os.Getenv("DARTS_O_MAT_TURNSTILE_SITE_KEY"), "site key of a Cloudflare Turnstile widget; with the secret key, registering needs the check (or $DARTS_O_MAT_TURNSTILE_SITE_KEY)")
		turnstileSecret = flag.String("turnstile-secret-key", os.Getenv("DARTS_O_MAT_TURNSTILE_SECRET_KEY"), "secret key of the Turnstile widget (or $DARTS_O_MAT_TURNSTILE_SECRET_KEY)")
	)
	flag.Parse()

	// SIGTERM is how a container or systemd stops the server, Ctrl-C in a terminal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	update.Start(ctx)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	// Forms and API requests are small; a limit keeps a huge body from filling the memory.
	r.Use(middleware.RequestSize(maxBodySize))
	// Refuses POSTs from other sites (by Sec-Fetch-Site, or Origin against Host), on top of
	// the SameSite cookie; the cameras and other clients without a browser aren't affected.
	r.Use(http.NewCrossOriginProtection().Handler)

	var (
		repo *store.Store
		err  error
	)
	switch *dbType {
	case "sqlite":
		repo, err = store.NewSQLite(ctx, *dbDSN)
	case "postgres":
		repo, err = store.NewPostgres(ctx, *dbDSN)
	default:
		log.Fatalf("unknown database %q, use sqlite or postgres", *dbType)
	}
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	opts := service.Options{MaxLoginFailures: *maxLoginFailures, ClientIPHeader: *clientIPHeader}
	if *dbType == "postgres" {
		// Replicas need PostgreSQL; its LISTEN/NOTIFY carries the live updates between them.
		opts.Bus = repo.Bus()
	}
	switch {
	case *turnstileSite != "" && *turnstileSecret != "":
		opts.Captcha = captcha.NewTurnstile(*turnstileSite, *turnstileSecret)
	case *turnstileSite != "" || *turnstileSecret != "":
		log.Fatal("turnstile needs both the site key and the secret key")
	}
	svc := service.New(repo, opts)
	if err := svc.ResumeTurnLimits(ctx); err != nil {
		log.Fatalf("failed to set the turn timers: %v", err)
	}
	go svc.Listen(ctx)
	r.Mount("/api", api.New(svc).Routes())
	r.Mount("/", controller.New(svc).Routes())

	serve(ctx, *addr, r)
}

// maxBodySize is the largest request body the server reads.
const maxBodySize = 1 << 20

// readHeaderTimeout is how long a client may take to send the headers of a request, so
// connections that send them slowly on purpose can't keep the server busy.
const readHeaderTimeout = 10 * time.Second

// shutdownTimeout is how long the requests that are running when the server stops get to
// finish, e.g. a dart on its way to the database.
const shutdownTimeout = 10 * time.Second

// serve serves the handler until ctx is done, then stops taking requests, lets the
// running ones finish and ends the live streams (the pages connect again by themselves).
func serve(ctx context.Context, addr string, h http.Handler) {
	streams := make(chan struct{})
	srv := &http.Server{
		Addr:    addr,
		Handler: h,
		// No ReadTimeout or WriteTimeout: they would also end the live streams (SSE).
		ReadHeaderTimeout: readHeaderTimeout,
		BaseContext:       func(net.Listener) context.Context { return sse.WithShutdown(context.Background(), streams) },
	}
	// HTTP/2 without TLS (h2c) besides HTTP/1.1, for a reverse proxy that speaks it to the
	// server (Caddy, Traefik, Envoy): the live streams share its connections. Browsers only
	// speak HTTP/2 over TLS, so they keep HTTP/1.1 here.
	srv.Protocols = new(http.Protocols)
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetUnencryptedHTTP2(true)
	srv.RegisterOnShutdown(func() { close(streams) })
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		<-ctx.Done()
		log.Print("stopping: finishing the running requests")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("stop the server: %v", err)
		}
	}()
	// Listen first, so the line saying the server runs only comes once the port is taken.
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("darts-o-mat %s is running on %s", buildinfo.Version(), pageURL(ln.Addr()))
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-stopped
}

// pageURL is the address of the web interface for the log, with localhost when the server
// listens on all interfaces.
func pageURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	if net.ParseIP(host).IsUnspecified() {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}
