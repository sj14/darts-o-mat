package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/api"
	"github.com/sj14/darts-o-mat/pkg/captcha"
	"github.com/sj14/darts-o-mat/pkg/frontend/controller"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
	"github.com/sj14/darts-o-mat/pkg/service"
)

func main() {
	var (
		dbType           = flag.String("db", "sqlite", "database to use: sqlite or postgres")
		dbDSN            = flag.String("dsn", "file:darts-o-mat.db", "data source name of the database")
		addr             = flag.String("addr", ":3000", "address to listen on")
		reg              = flag.Bool("registration", true, "allow anyone to create an account")
		maxLoginFailures = flag.Int("max-login-failures", 10, "failed logins a username may have within 15 minutes before it is locked for a while (a network may have 5 times as many); 0 is no limit")
		clientIPHeader   = flag.String("client-ip-header", "", "behind a proxy: the header it passes the client's address in, e.g. X-Forwarded-For; only if all requests come through it")
		// The secret key is better passed in the environment: flags show up in the process list.
		turnstileSite   = flag.String("turnstile-site-key", os.Getenv("DARTS_O_MAT_TURNSTILE_SITE_KEY"), "site key of a Cloudflare Turnstile widget; with the secret key, registering needs the check (or $DARTS_O_MAT_TURNSTILE_SITE_KEY)")
		turnstileSecret = flag.String("turnstile-secret-key", os.Getenv("DARTS_O_MAT_TURNSTILE_SECRET_KEY"), "secret key of the Turnstile widget (or $DARTS_O_MAT_TURNSTILE_SECRET_KEY)")
	)
	flag.Parse()

	ctx := context.Background()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)

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

	opts := service.Options{Registration: *reg, MaxLoginFailures: *maxLoginFailures, ClientIPHeader: *clientIPHeader}
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
	r.Mount("/api", api.New(svc).Routes())
	r.Mount("/", controller.New(svc).Routes())

	log.Fatal(http.ListenAndServe(*addr, r))
}
