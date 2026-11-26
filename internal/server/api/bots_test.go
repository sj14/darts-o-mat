package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/service"
	"github.com/sj14/darts-o-mat/internal/server/store"
	"golang.org/x/crypto/bcrypt"
)

// A bot is added to a game's lobby with its spread; the game shows it at its seat.
func TestBotAPI(t *testing.T) {
	t.Parallel()
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	h := New(service.New(st, service.Options{PasswordCost: bcrypt.MinCost})).Routes()
	anna := newClient(t, h, "anna")

	body, _ := json.Marshal(map[string]any{"mode": "x01", "rules": map[string]any{"start_score": 501, "out_mode": "double"}})
	created := anna.expect(http.StatusCreated, "POST", "/games", string(body))
	id := strconv.Itoa(int(created["game"].(map[string]any)["id"].(float64)))

	anna.expect(http.StatusBadRequest, "POST", "/games/"+id+"/bots", `{"spread_um": 0}`)
	anna.expect(http.StatusBadRequest, "POST", "/games/"+id+"/bots", `{"spread_um": 3999}`)
	v := anna.expect(http.StatusOK, "POST", "/games/"+id+"/bots", `{"spread_um": 12500}`)
	participants := v["game"].(map[string]any)["participants"].([]any)
	if len(participants) != 2 || participants[1].(map[string]any)["bot_spread_um"] != 12500.0 {
		t.Fatalf("participants %v, want anna and a bot with a spread of 12500 µm", participants)
	}
	if players := v["players"].([]any); players[1].(map[string]any)["bot"] != true {
		t.Errorf("second player %v, want a bot", players[1])
	}
}
