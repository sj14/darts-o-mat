package camapp

import (
	"testing"
	"time"
)

// A failed detection reconnects its cameras once; the detection started by that only
// reconnects again if it ran for a while, so a camera that keeps failing doesn't make the
// board reopen its cameras in a loop.
func TestReconnectAfter(t *testing.T) {
	t.Parallel()
	start := time.Now()
	for _, tt := range []struct {
		name        string
		reconnected bool
		ranFor      time.Duration
		want        bool
	}{
		{"started by the user", false, time.Second, true},
		{"reconnected, failed again at once", true, time.Second, false},
		{"reconnected, failed after a while", true, reconnectWithin, true},
	} {
		r := &run{started: start, reconnected: tt.reconnected}
		if got := r.reconnectAfter(start.Add(tt.ranFor)); got != tt.want {
			t.Errorf("%s: reconnect %v, want %v", tt.name, got, tt.want)
		}
	}
}
