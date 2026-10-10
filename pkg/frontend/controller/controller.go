package controller

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/frontend/static"
	"github.com/sj14/darts-o-mat/pkg/service"
)

type Controller struct {
	svc *service.Service
}

func New(svc *service.Service) *Controller {
	return &Controller{svc: svc}
}

func (c *Controller) Routes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(static.FS)))
	r.Get("/", c.home)
	r.Post("/players", c.createPlayer)
	r.Post("/games", c.createGame)
	r.Get("/games/{id}", c.showGame)
	r.Post("/games/{id}/darts", c.addDart)
	r.Post("/games/{id}/undo", c.undoDart)
	return r
}

// gameID parses the {id} URL parameter and responds with 404 if that fails.
func gameID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	return id, true
}

func render(w http.ResponseWriter, r *http.Request, status int, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := c.Render(r.Context(), w); err != nil {
		log.Printf("render: %v", err)
	}
}

func serverError(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	http.Error(w, "Interner Fehler", http.StatusInternalServerError)
}

func isNotFound(err error) bool {
	return errors.Is(err, service.ErrNotFound)
}
