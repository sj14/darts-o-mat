package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/api"
	"github.com/sj14/darts-o-mat/pkg/frontend/controller"
	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

func main() {
	var (
		dbType = flag.String("db", "sqlite", "database to use: sqlite or postgres")
		dbDSN  = flag.String("dsn", "file:darts-o-mat.db", "data source name of the database")
		addr   = flag.String("addr", ":3000", "address to listen on")
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

	ctrl := controller.New(repo)

	r.Get("/games/x01", ctrl.ShowNewGameX01)
	r.Post("/games/x01", ctrl.PostNewGameX01)
	r.Mount("/api", api.New(repo).Routes())

	log.Fatal(http.ListenAndServe(*addr, r))
}
