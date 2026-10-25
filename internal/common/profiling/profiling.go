// Package profiling serves Go's profiles (net/http/pprof: CPU, memory, goroutines,
// blocking, traces) on an address of its own, to find out where a program spends its time
// and memory, e.g. with
//
//	go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
//
// Off by default: the profiles show the program's internals to whoever reaches the
// address, without a login, so it belongs on localhost (or a port nobody else reaches).
package profiling

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"time"
)

// Usage is the help of the -pprof flag that sets the address of Start.
const Usage = "address to serve Go's profiles on (/debug/pprof/), e.g. localhost:6060; off when empty. Without a login: keep it on localhost"

// Handler serves the profiles under /debug/pprof/.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}

// Start serves the profiles on addr until ctx is done; an empty addr serves nothing. It
// returns once it listens, or the error if it can't.
func Start(ctx context.Context, addr string) error {
	if addr == "" {
		return nil
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	// No WriteTimeout: a CPU profile or a trace takes as many seconds as asked for.
	srv := &http.Server{Handler: Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("profiling: %v", err)
		}
	}()
	log.Printf("profiles on http://%s/debug/pprof/", ln.Addr())
	return nil
}
