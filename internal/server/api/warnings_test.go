package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/server/service"
	"github.com/sj14/darts-o-mat/internal/server/store"
	"golang.org/x/crypto/bcrypt"
)

// darts-o-cam reports a board's warnings with its token; bodies that are too large or
// hold too many warnings are refused before they reach the database.
func TestBoardWarningsAPI(t *testing.T) {
	t.Parallel()
	st, err := store.NewSQLite(context.Background(), "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() }) // the in-memory database is gone with it, also for -count
	h := New(service.New(st, service.Options{PasswordCost: bcrypt.MinCost})).Routes()
	anna := newClient(t, h, "anna")
	b := anna.expect(http.StatusCreated, "POST", "/boards", `{"name":"Keller"}`)
	boardID := strconv.FormatInt(int64(b["id"].(float64)), 10)
	created := anna.expect(http.StatusCreated, "POST", "/boards/"+boardID+"/token", ``)
	token := created["token"].(string)

	put := func(body string) int {
		t.Helper()
		req := httptest.NewRequest("PUT", "/board/warnings", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := put(`{"warnings":[{"code":"grid_mismatch","details":{"camera":"cam2"}}]}`); code != http.StatusNoContent {
		t.Errorf("one warning: status %d, want %d", code, http.StatusNoContent)
	}
	boards := anna.do("GET", "/boards", ``).Body.String()
	if !strings.Contains(boards, `"warnings":[{"code":"grid_mismatch","details":{"camera":"cam2"}}]`) {
		t.Errorf("boards %s: want the warning", boards)
	}

	tooMany := `{"warnings":[` + strings.Repeat(`{"code":"x"},`, service.MaxWarnings) + `{"code":"x"}]}`
	if code := put(tooMany); code != http.StatusBadRequest {
		t.Errorf("%d warnings: status %d, want %d", service.MaxWarnings+1, code, http.StatusBadRequest)
	}
	huge := `{"warnings":[{"code":"` + strings.Repeat("x", maxWarningsBody) + `"}]}`
	if code := put(huge); code != http.StatusRequestEntityTooLarge {
		t.Errorf("body over %d bytes: status %d, want %d", maxWarningsBody, code, http.StatusRequestEntityTooLarge)
	}
}
