// Package auth connects HTTP requests to the logged-in account via a session cookie.
package auth

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/domain/account"
	"github.com/sj14/darts-o-mat/pkg/service"
)

const cookieName = "darts_session"

type ctxKey struct{}

// Account returns the logged-in account of a request that passed Require.
func Account(ctx context.Context) account.Account {
	a, _ := ctx.Value(ctxKey{}).(account.Account)
	return a
}

// Require only lets requests with a valid session through and stores their account in the context.
// Other requests are answered by unauthenticated.
func Require(svc *service.Service, unauthenticated http.HandlerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil {
				unauthenticated(w, r)
				return
			}
			a, err := svc.Authenticate(r.Context(), cookie.Value)
			if errors.Is(err, service.ErrUnauthenticated) {
				unauthenticated(w, r)
				return
			}
			if err != nil {
				log.Printf("authenticate: %v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, a)))
		})
	}
}

// Login checks the credentials and sets the session cookie.
func Login(w http.ResponseWriter, r *http.Request, svc *service.Service, username, password string) (account.Account, error) {
	token, a, err := svc.Login(r.Context(), username, password)
	if err != nil {
		return a, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(service.SessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPS(r),
		// Lax keeps the cookie out of cross-site POST requests, which protects against CSRF.
		SameSite: http.SameSiteLaxMode,
	})
	return a, nil
}

// Logout ends the session and removes the cookie.
func Logout(w http.ResponseWriter, r *http.Request, svc *service.Service) error {
	if cookie, err := r.Cookie(cookieName); err == nil {
		if err := svc.Logout(r.Context(), cookie.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: isHTTPS(r), SameSite: http.SameSiteLaxMode})
	return nil
}

// isHTTPS also detects HTTPS terminated by a reverse proxy.
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}
