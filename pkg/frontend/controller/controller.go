package controller

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/auth"
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
	r.Get("/login", c.showLogin)
	r.Post("/login", c.login)
	r.Get("/register", c.showRegister)
	r.Post("/register", c.register)
	r.Post("/logout", c.logout)

	r.Group(func(r chi.Router) {
		r.Use(auth.Require(c.svc, redirectToLogin))

		r.Get("/", c.home)
		r.Post("/guests", c.createGuest)
		r.Get("/boards", c.showBoards)
		r.Post("/boards", c.createBoard)
		r.Post("/boards/{id}/delete", c.deleteBoard)

		r.Post("/games", c.createGame)
		r.Get("/games/{id}", c.showGame)
		r.Get("/games/{id}/board", c.showBoard)
		r.Post("/games/{id}/darts", c.addDart)
		r.Post("/games/{id}/undo", c.undoDart)
		r.Post("/games/{id}/accept", c.acceptInvitation)
		r.Post("/games/{id}/decline", c.declineInvitation)
		r.Post("/games/{id}/invitations/{playerID}/withdraw", c.withdrawInvitation)
	})
	return r
}

// redirectToLogin sends logged-out users to the login page.
// htmx requests get the HX-Redirect header instead, so the login page isn't swapped into the current page.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// urlID parses a numeric URL parameter and responds with 404 if that fails.
func urlID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
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
