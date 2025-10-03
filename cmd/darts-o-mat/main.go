package main

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/domain/game"
	"github.com/sj14/darts-o-mat/pkg/domain/player"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/frontend/controller"
	"github.com/sj14/darts-o-mat/pkg/repos/sqlite"
	"gorm.io/gorm"
)

func main() {
	game301 := game.GameX01{
		HitCounter:    20,
		CurrentPlayer: 0,
		OutMode:       game.OutModeDouble,
		Players: []*game.PlayerState{
			{Player: player.Player{Model: gorm.Model{ID: 0}}},
			{Player: player.Player{Model: gorm.Model{ID: 1}}},
		},
	}

	game301.RegisterDart(throw.Dart{Number: 10})
	game301.RegisterDart(throw.Dart{Number: 8})
	game301.RegisterDart(throw.Dart{Number: 1})
	game301.RegisterDart(throw.Dart{Number: 10})
	game301.RegisterDart(throw.Dart{Number: 0})
	game301.RegisterDart(throw.Dart{Number: 5, Multiplicator: 2})
	// game301.Undo()
	// game301.Abort()
	// game301.Status()

	// log.Printf("state: %+v\n", game301)
	// log.Printf("state 1: %+v\n", game301.Players[0].State)
	// log.Printf("state 2: %+v\n", game301.Players[1].State)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)

	store := sqlite.NewSQLiteStore()
	ctx := context.Background()

	store.StorePlayer(ctx, &player.Player{Name: "Simon"})
	store.StorePlayer(ctx, &player.Player{Name: "Mario"})

	ctrl := controller.New(store)

	r.Get("/games/x01", ctrl.ShowNewGameX01)
	r.Post("/games/x01", ctrl.PostNewGameX01)

	http.ListenAndServe(":3000", r)
}
