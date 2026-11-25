package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/domain/account"
	"github.com/sj14/darts-o-mat/internal/server/domain/bot"
	"github.com/sj14/darts-o-mat/internal/server/domain/game"
	"github.com/sj14/darts-o-mat/internal/server/domain/game/x01"
	"github.com/sj14/darts-o-mat/internal/server/service"
	"github.com/sj14/darts-o-mat/internal/server/store"
	"golang.org/x/crypto/bcrypt"
)

// botSite is a server with one logged-in account and its game's lobby.
type botSite struct {
	h      http.Handler
	svc    *service.Service
	acc    account.Account
	gameID int64
	cookie *http.Cookie
}

func newBotSite(t *testing.T) *botSite {
	t.Helper()
	ctx := context.Background()
	st, err := store.NewSQLite(ctx, "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	svc := service.New(st, service.Options{PasswordCost: bcrypt.MinCost})
	acc, err := svc.Register(ctx, "anna", "", "", "password")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := svc.Login(ctx, "anna", "password")
	if err != nil {
		t.Fatal(err)
	}
	g := game.Game{Settings: game.Settings{Mode: game.ModeX01, Rules: &x01.Settings{StartScore: 501, OutMode: x01.OutModeDouble}}}
	v, err := svc.CreateGame(ctx, acc, g, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &botSite{
		h: New(svc).Routes(), svc: svc, acc: acc, gameID: v.Game.ID,
		cookie: &http.Cookie{Name: "darts_session", Value: token},
	}
}

func (s *botSite) do(method, target string, form url.Values) *httptest.ResponseRecorder {
	body := strings.NewReader("")
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, target, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	r.AddCookie(s.cookie)
	w := httptest.NewRecorder()
	s.h.ServeHTTP(w, r)
	return w
}

// spreads returns the bots' spreads in the game's lobby.
func (s *botSite) spreads(t *testing.T) []game.Spread {
	t.Helper()
	v, err := s.svc.Game(context.Background(), s.acc, s.gameID)
	if err != nil {
		t.Fatal(err)
	}
	var spreads []game.Spread
	for _, p := range v.Game.Participants {
		if p.BotSpread > 0 {
			spreads = append(spreads, p.BotSpread)
		}
	}
	return spreads
}

// The slider's position seats a bot with its spread; a missing or no number adds nothing and
// says why.
func TestAddBotFromForm(t *testing.T) {
	t.Parallel()
	s := newBotSite(t)
	botsURL := "/games/" + strconv.FormatInt(s.gameID, 10) + "/bots"

	for _, form := range []url.Values{{}, {"position": {"far"}}} {
		w := s.do(http.MethodPost, botsURL, form)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "must be between") {
			t.Errorf("form %v: %d, want 409 with the message about the spread", form, w.Code)
		}
		if got := s.spreads(t); len(got) != 0 {
			t.Fatalf("form %v made bots %v, want none", form, got)
		}
	}

	w := s.do(http.MethodPost, botsURL, url.Values{"position": {"500"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("valid position: %d, want 303", w.Code)
	}
	if got, want := s.spreads(t), []game.Spread{bot.SpreadAt(500)}; len(got) != 1 || got[0] != want[0] {
		t.Errorf("bots %v, want %v", got, want)
	}
}

// The slider's value is answered for every mode; a mode or a position that doesn't exist is a
// 400.
func TestBotStrength(t *testing.T) {
	t.Parallel()
	s := newBotSite(t)
	tests := []struct {
		query string
		code  int
		want  string
	}{
		{"mode=x01&position=500", http.StatusOK, " average"},
		{"mode=around_the_clock&position=500", http.StatusOK, " hits"},
		{"mode=cricket&position=500", http.StatusOK, " MPR"},
		{"mode=darts&position=500", http.StatusBadRequest, ""},
		{"position=500", http.StatusBadRequest, ""},
		{"mode=x01&position=far", http.StatusBadRequest, ""},
		{"mode=x01", http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()
			w := s.do(http.MethodGet, "/bots/strength?"+tt.query, nil)
			if w.Code != tt.code || !strings.Contains(w.Body.String(), tt.want) {
				t.Errorf("%d %q, want %d with %q", w.Code, w.Body, tt.code, tt.want)
			}
		})
	}
}
