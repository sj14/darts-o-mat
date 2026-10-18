package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/repos/store"
	"github.com/sj14/darts-o-mat/pkg/service"
)

// client calls the API as one account, with its session cookie.
type client struct {
	t       *testing.T
	h       http.Handler
	cookies []*http.Cookie
}

func (c *client) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	for _, ck := range c.cookies {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.h.ServeHTTP(rec, req)
	return rec
}

func (c *client) expect(status int, method, path, body string) map[string]any {
	c.t.Helper()
	rec := c.do(method, path, body)
	if rec.Code != status {
		c.t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, status, rec.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

func newClient(t *testing.T, h http.Handler, username string) *client {
	t.Helper()
	c := &client{t: t, h: h}
	c.expect(http.StatusCreated, "POST", "/register", `{"username":"`+username+`","password":"geheim12345"}`)
	rec := c.do("POST", "/login", `{"username":"`+username+`","password":"geheim12345"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", username, rec.Code, rec.Body)
	}
	c.cookies = rec.Result().Cookies()
	return c
}

func TestTournamentAPI(t *testing.T) {
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	h := New(service.New(st, service.Options{Registration: true})).Routes()
	anna, ben := newClient(t, h, "anna"), newClient(t, h, "ben")
	anna.expect(http.StatusNoContent, "POST", "/friends", `{"username":"ben"}`)
	ben.expect(http.StatusNoContent, "POST", "/friends/1/accept", ``)

	// A tournament of anna, her guest and ben, who is invited.
	guest := anna.expect(http.StatusCreated, "POST", "/players", `{"name":"Papa"}`)
	guestID := int64(guest["id"].(float64))
	body, _ := json.Marshal(map[string]any{
		"format": "round_robin", "mode": "x01", "legs": 1, "player_ids": []int64{1, guestID}, "invite": []string{"ben"},
		"rules": map[string]any{"start_score": 40, "out_mode": "single"},
	})
	created := anna.expect(http.StatusCreated, "POST", "/tournaments", string(body))
	tour := created["tournament"].(map[string]any)
	id := int64(tour["id"].(float64))
	if tour["format"] != "round_robin" || tour["rules"] == nil {
		t.Errorf("created tournament = %v", tour)
	}
	path := "/tournaments/" + itoa(id)

	// Ben sees the invitation, and the games wait for him.
	if list := ben.do("GET", "/tournaments", ""); list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"invited":true`)) {
		t.Errorf("ben's tournaments: %d %s", list.Code, list.Body)
	}
	anna.expect(http.StatusConflict, "POST", path+"/matches/0", ``)
	ben.expect(http.StatusOK, "POST", path+"/accept", ``)
	view := anna.expect(http.StatusOK, "GET", path, ``)
	if view["ready"] != true {
		t.Fatalf("not ready after ben accepted: %v", view)
	}

	// Anna starts the first match she plays; the answer is its game.
	game := anna.expect(http.StatusCreated, "POST", path+"/matches/0", ``)
	if game["game"] == nil {
		t.Errorf("starting a match: %v", game)
	}
	// A stranger sees nothing of it.
	carla := newClient(t, h, "carla")
	carla.expect(http.StatusNotFound, "GET", path, ``)

	// Leaving ends the running tournament for everyone.
	left := ben.expect(http.StatusOK, "POST", path+"/leave", ``)
	if left["tournament"].(map[string]any)["aborted_at"] == nil {
		t.Errorf("after leaving: %v", left)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
