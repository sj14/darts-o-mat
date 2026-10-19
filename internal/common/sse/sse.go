// Package sse streams Server-Sent Events, which htmx's SSE extension swaps into a page.
package sse

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// keepalive is how often a comment is sent on an idle stream, so proxies don't close it.
const keepalive = 25 * time.Second

// Stream sends the event whenever changes receives a value, until the request ends:
// render returns its data, e.g. a rendered fragment of the page; an error ends the
// stream (e.g. what it shows is gone).
func Stream(w http.ResponseWriter, r *http.Request, event string, changes <-chan struct{}, render func() (string, error)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // don't let a reverse proxy hold events back
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	tick := time.NewTicker(keepalive)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			fmt.Fprint(w, ": keepalive\n\n")
		case <-changes:
			data, err := render()
			if err != nil {
				return
			}
			write(w, event, data)
		}
		flusher.Flush()
	}
}

// write writes one event; every line of the data needs its own "data:" field.
func write(w io.Writer, event, data string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for line := range strings.SplitSeq(data, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
