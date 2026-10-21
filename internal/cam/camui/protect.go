package camui

import (
	"net"
	"net/http"
	"strings"
)

// protect guards the web interface, which has no login, against other web pages open in
// the user's browser: a form of another site can't change anything (cross-origin
// requests are refused, http.CrossOriginProtection), and a page can't read it under a
// domain of its own that resolves to this computer (DNS rebinding, see localHost).
func protect(next http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(localHost(next))
}

// localSuffixes are the endings of names that only exist in a local network, never as a
// public domain: an attacker can't make one of them resolve to this computer.
var localSuffixes = []string{".localhost", ".local", ".lan", ".home.arpa", ".internal"}

// localHost lets a request through only if its Host is this computer or a name in the
// local network: an IP address, localhost, a name without a dot (http://raspberrypi:8090)
// or one ending in a local suffix (raspberrypi.local). Otherwise a public domain that
// resolves to this computer could read the pages, the camera images and the server token.
func localHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isLocalHost(r.Host) {
			http.Error(w, "Open darts-o-cam by the address of its computer, e.g. http://localhost:8090.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isLocalHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport // no port
	}
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	switch {
	case host == "":
		return false
	case net.ParseIP(host) != nil, host == "localhost", !strings.Contains(host, "."):
		return true
	}
	for _, s := range localSuffixes {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	return false
}
