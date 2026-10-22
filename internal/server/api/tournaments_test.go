package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/service"
	"github.com/sj14/darts-o-mat/internal/server/store"
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

	// A tournament of anna, her guest and ben, who is invited: made in its lobby.
	guest := anna.expect(http.StatusCreated, "POST", "/players", `{"name":"Papa"}`)
	guestID := int64(guest["id"].(float64))
	body, _ := json.Marshal(map[string]any{
		"format": "round_robin", "mode": "x01", "legs": 1,
		"rules": map[string]any{"start_score": 40, "out_mode": "single"},
	})
	created := anna.expect(http.StatusCreated, "POST", "/tournaments", string(body))
	tour := created["tournament"].(map[string]any)
	id := int64(tour["id"].(float64))
	if tour["format"] != "round_robin" || tour["rules"] == nil || created["started"] != false {
		t.Errorf("created tournament = %v", created)
	}
	path := "/tournaments/" + itoa(id)
	anna.expect(http.StatusOK, "POST", path+"/players", `{"player_id":`+itoa(guestID)+`}`)
	anna.expect(http.StatusOK, "POST", path+"/invitations", `{"username":"ben"}`)

	// Ben sees the invitation, and the games wait for him.
	if list := ben.do("GET", "/tournaments", ""); list.Code != http.StatusOK || !bytes.Contains(list.Body.Bytes(), []byte(`"invited":true`)) {
		t.Errorf("ben's tournaments: %d %s", list.Code, list.Body)
	}
	anna.expect(http.StatusConflict, "POST", path+"/start", ``)
	ben.expect(http.StatusOK, "POST", path+"/accept", ``)
	if view := anna.expect(http.StatusOK, "POST", path+"/start", ``); view["started"] != true {
		t.Fatalf("not started after ben accepted and anna started it: %v", view)
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

func TestOpenLobbiesAPI(t *testing.T) {
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	h := New(service.New(st, service.Options{Registration: true})).Routes()
	// A player has one seat at a time: everyone takes part in one lobby only.
	anna, ben, carla := newClient(t, h, "anna"), newClient(t, h, "ben"), newClient(t, h, "carla")
	dora, erik := newClient(t, h, "dora"), newClient(t, h, "erik")

	rules := map[string]any{"start_score": 301, "out_mode": "double"}
	game, _ := json.Marshal(map[string]any{"mode": "x01", "legs": 1, "seats": 2, "rules": rules})
	tour, _ := json.Marshal(map[string]any{"format": "round_robin", "mode": "x01", "legs": 1, "seats": 3, "rules": rules})
	g := anna.expect(http.StatusCreated, "POST", "/games", string(game))
	gameID := itoa(int64(g["game"].(map[string]any)["id"].(float64)))
	tr := carla.expect(http.StatusCreated, "POST", "/tournaments", string(tour))
	tourID := itoa(int64(tr["tournament"].(map[string]any)["id"].(float64)))

	var lobbies []map[string]any
	rec := ben.do("GET", "/lobbies", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &lobbies); err != nil || len(lobbies) != 2 {
		t.Fatalf("ben's lobbies: %v, %s", err, rec.Body)
	}
	rec = anna.do("GET", "/lobbies", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &lobbies); err != nil || len(lobbies) != 1 || lobbies[0]["tournament"] != true {
		t.Errorf("anna's lobbies: %v, %s; want only carla's tournament", err, rec.Body)
	}

	// The game takes its last seat with ben and starts; then it is full.
	if v := ben.expect(http.StatusOK, "POST", "/games/"+gameID+"/join", ``); v["started"] != true {
		t.Errorf("after joining: %v", v)
	}
	dora.expect(http.StatusBadRequest, "POST", "/games/"+gameID+"/join", ``)

	// The tournament needs a third player.
	if v := dora.expect(http.StatusOK, "POST", "/tournaments/"+tourID+"/join", ``); v["started"] == true {
		t.Errorf("ready with a seat free: %v", v)
	}
	if v := erik.expect(http.StatusOK, "POST", "/tournaments/"+tourID+"/join", `{}`); v["started"] != true {
		t.Errorf("not ready after the last seat: %v", v)
	}
}
