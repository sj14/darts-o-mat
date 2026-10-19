// Package update finds out whether a newer release of the project exists, for the footers
// of the server's and darts-o-cam's web interfaces.
//
// It asks GitHub for the latest release when the program starts and then once a day, in the
// background. A failed check is tried again in an hour and shows nothing; a binary that
// isn't a release (the version is a git revision) never shows an update.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sj14/darts-o-mat/common/buildinfo"
)

const (
	interval = 24 * time.Hour
	retry    = time.Hour
)

var (
	// apiURL is where the latest release is asked for (a variable for the test).
	apiURL = "https://api.github.com/repos/sj14/darts-o-mat/releases/latest"
	client = &http.Client{Timeout: 10 * time.Second}

	mu     sync.RWMutex
	latest string // the newest release's version without the "v", empty until known
)

// Start checks for the latest release now and then every day, in the background until ctx
// ends. It is called by the programs' main functions only, so tests make no request.
func Start(ctx context.Context) {
	go func() {
		for {
			wait := interval
			if err := check(ctx); err != nil {
				wait = retry
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
}

// check asks for the latest release (GitHub's "latest" is the newest that isn't a
// pre-release) and remembers its version.
func check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "darts-o-mat/"+buildinfo.Version())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("latest release: %s", resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}
	if release.TagName == "" {
		return fmt.Errorf("latest release: no tag")
	}
	mu.Lock()
	latest = strings.TrimPrefix(release.TagName, "v")
	mu.Unlock()
	return nil
}

// Available returns the version of the latest release if it is newer than the running
// program, else false.
func Available() (string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	return latest, newer(latest, buildinfo.Version())
}

// newer reports whether the release version latest is newer than current. Anything that
// isn't a version (a git revision, nothing yet) is never older.
func newer(latest, current string) bool {
	l, ok1 := parse(latest)
	c, ok2 := parse(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l.numbers {
		if l.numbers[i] != c.numbers[i] {
			return l.numbers[i] > c.numbers[i]
		}
	}
	// The same numbers: a pre-release (1.2.3-rc.1) is older than the release.
	return c.pre != "" && l.pre == ""
}

type version struct {
	numbers [3]int
	pre     string
}

// parse reads 1.2.3, v1.2.3 and 1.2.3-rc.1.
func parse(s string) (version, bool) {
	s = strings.TrimPrefix(s, "v")
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var v version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return version{}, false
		}
		v.numbers[i] = n
	}
	v.pre = pre
	return v, true
}
