package view

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// The waiting page asks once, after the delay the server worked out.
func TestQuickMatchCheckAsksAfterTheDelay(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	if err := QuickMatchCheck(7, 91500*time.Millisecond).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); !strings.Contains(got, `hx-post="/games/7/quick"`) || !strings.Contains(got, `hx-trigger="load delay:91s"`) {
		t.Errorf("got %s, want a post to /games/7/quick after 91 s", got)
	}
}
