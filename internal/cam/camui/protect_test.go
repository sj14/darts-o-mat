package camui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Other web pages open in the browser can't change anything, and the interface only
// answers under this computer's address or a local name.
func TestProtect(t *testing.T) {
	h := protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for _, tt := range []struct {
		name, method, host string
		header             map[string]string
		want               int
	}{
		{"page", "GET", "localhost:8090", nil, http.StatusOK},
		{"form of the page itself", "POST", "localhost:8090", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://localhost:8090"}, http.StatusOK},
		{"form of another site", "POST", "localhost:8090", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, http.StatusForbidden},
		{"older browser, other origin", "POST", "localhost:8090", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"curl", "POST", "localhost:8090", nil, http.StatusOK},
		{"IPv4", "GET", "192.168.1.20:8090", nil, http.StatusOK},
		{"IPv6", "GET", "[::1]:8090", nil, http.StatusOK},
		{"computer name", "GET", "raspberrypi:8090", nil, http.StatusOK},
		{"mDNS name", "GET", "raspberrypi.local:8090", nil, http.StatusOK},
		{"public domain resolving here", "GET", "rebind.evil.example:8090", nil, http.StatusForbidden},
		{"public domain, trailing dot", "GET", "evil.example.:8090", nil, http.StatusForbidden},
	} {
		r := httptest.NewRequest(tt.method, "/board/1/settings", nil)
		r.Host = tt.host
		for k, v := range tt.header {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Errorf("%s: status %d, want %d", tt.name, w.Code, tt.want)
		}
	}
}
