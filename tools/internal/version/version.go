// Package version determines the version of the packages built by tools/linuxpkg,
// tools/winpkg and tools/macapp from git and stamps it into their binaries.
package version

import (
	"os/exec"
	"regexp"
	"strings"
)

// FromGit returns the version from the latest git tag of the checkout in dir, e.g. 1.2.3
// for v1.2.3. Without such a tag, or with commits or changes since, it is marked as a
// development build: 0.0.0+git.abc1234 (dpkg and rpm sort that before 0.0.1).
func FromGit(dir string) string {
	describe := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	describe.Dir = dir
	desc, err := describe.Output()
	if err != nil {
		return "0.0.0+unknown"
	}
	d := strings.TrimSpace(string(desc))
	if m := regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`).FindStringSubmatch(d); m != nil {
		return m[1]
	}
	return "0.0.0+git." + strings.NewReplacer("-", ".").Replace(d)
}

// StampFlag is the -ldflags argument that sets version in the binary's buildinfo, which
// its footer shows and the hint about a newer release compares against. Without it the
// binary tells the git revision it was built from.
func StampFlag(version string) string {
	return "-X github.com/sj14/darts-o-mat/internal/common/buildinfo.stamped=" + version
}
