package controller

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/common/i18n"
	"github.com/sj14/darts-o-mat/internal/common/static"
	"github.com/sj14/darts-o-mat/internal/server/auth"
	"github.com/sj14/darts-o-mat/internal/server/domain/page"
	"github.com/sj14/darts-o-mat/internal/server/frontend/view"
	"github.com/sj14/darts-o-mat/internal/server/service"
)

type Controller struct {
	svc *service.Service
}

func New(svc *service.Service) *Controller {
	return &Controller{svc: svc}
}

func (c *Controller) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(i18n.Middleware)
	r.Use(c.withPages)
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(static.FS)))
	r.Get("/login", c.showLogin)
	r.Post("/login", c.login)
	r.Get("/register", c.showRegister)
	r.Post("/register", c.register)
	r.Post("/logout", c.logout)

	// Pages for everyone: with the account of whoever is logged in, without one for the others.
	r.Group(func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler { return auth.Require(c.svc, next.ServeHTTP)(next) })
		r.Get("/about", c.about)
		for _, name := range page.Names {
			r.Get(view.PagePath(name), c.showPage(name))
		}
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Require(c.svc, c.loggedOut))

		r.Get("/", c.home)
		r.Post("/language", c.setLanguage)
		r.Get("/settings", c.showSettings)
		r.Post("/settings/color", c.setColor)
		r.Post("/settings/email", c.setEmail)
		r.Post("/settings/invisible", c.setInvisible)
		r.Post("/settings/delete", c.deleteAccount)
		r.Get("/admin", c.showAdmin)
		r.Post("/admin/pages/{name}", c.setPage)
		r.Post("/admin/registration", c.setRegistration)
		r.Post("/admin/admins", c.setAdmin)
		r.Post("/guests/{id}/delete", c.deleteGuest)
		r.Get("/stats", c.showStats)
		r.Get("/friends", c.showFriends)
		r.Post("/friends", c.requestFriend)
		r.Post("/friends/{id}/accept", c.acceptFriend)
		r.Post("/friends/{id}/remove", c.removeFriend)
		r.Get("/boards", c.showBoards)
		r.Post("/boards", c.createBoard)
		r.Post("/boards/{id}/delete", c.deleteBoard)
		r.Post("/boards/{id}/token", c.createBoardToken)

		r.Get("/lobbies", c.showLobbies)
		r.Post("/games", c.createGame)
		c.lobbyRoutes(r, "/games", service.GameLobby)
		c.lobbyRoutes(r, "/tournaments", service.TournamentLobby)
		r.Get("/tournaments/{id}", c.showTournament)
		r.Get("/tournaments/{id}/events", c.tournamentEvents)
		r.Post("/tournaments/{id}/matches/{match}", c.startMatch)
		r.Post("/tournaments/{id}/leave", c.leaveTournament)
		r.Get("/games/{id}", c.showGame)
		r.Get("/games/{id}/events", c.gameEvents)
		r.Post("/games/{id}/darts", c.addDart)
		r.Get("/games/{id}/board", c.showBoard)
		r.Get("/games/{id}/keypad", c.showKeypad)
		r.Get("/games/{id}/darts/{dartID}", c.showEditDart)
		r.Post("/games/{id}/darts/{dartID}", c.editDart)
		r.Post("/games/{id}/undo", c.undoDart)
		r.Post("/games/{id}/takeout", c.takeout)
		r.Post("/games/{id}/ratings/{playerID}", c.ratePlayer)
		r.Post("/games/{id}/leave", c.leaveGame)
		r.Post("/games/{id}/board", c.useBoard)
		r.Post("/games/{id}/bull-off/{playerID}", c.decideBullOff)
	})
	return r
}

// loggedOut answers requests of logged-out users: the start page shows the landing page,
// everything else sends them to the login page.
// htmx requests get the HX-Redirect header instead, so the login page isn't swapped into the current page.
func (c *Controller) loggedOut(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		c.about(w, r)
		return
	}
	if r.Header.Get("HX-Request") != "" {
		w.Header().Set("HX-Redirect", "/login")
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// about shows the landing page, also to logged-in accounts, e.g. to find darts-o-cam's
// download.
func (c *Controller) about(w http.ResponseWriter, r *http.Request) {
	open, err := c.svc.RegistrationOpen(r.Context())
	if err != nil {
		serverError(w, err)
		return
	}
	render(w, r, http.StatusOK, view.Landing(open, auth.Account(r.Context()).Username))
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
	http.Error(w, "Internal error", http.StatusInternalServerError)
}

func isNotFound(err error) bool {
	return errors.Is(err, service.ErrNotFound)
}

// setLanguage stores the language the account chose (empty: the browser's) and goes
// back to the page it was chosen on.
func (c *Controller) setLanguage(w http.ResponseWriter, r *http.Request) {
	_, err := c.svc.SetLanguage(r.Context(), auth.Account(r.Context()), r.FormValue("language"))
	if err != nil && !errors.Is(err, service.ErrInvalid) {
		serverError(w, err)
		return
	}
	back := "/"
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host {
		back = ref.RequestURI()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
