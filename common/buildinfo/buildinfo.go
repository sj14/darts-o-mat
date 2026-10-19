// Package buildinfo tells which version of the project a binary was built from, for the
// footers of the server's and darts-o-cam's web interfaces.
package buildinfo

import "runtime/debug"

// ProjectURL is the project's page.
const ProjectURL = "https://github.com/sj14/darts-o-mat"

// stamped is the version a release build sets: -ldflags "-X
// github.com/sj14/darts-o-mat/common/buildinfo.stamped=1.2.3". The container image is built
// without git, so it needs it.
var stamped string

// Version returns the version the binary was built from: the one stamped in at build time,
// else the short git revision, marked "-dirty" with uncommitted changes; for a binary from
// go install the module version (v1.2.3). Empty if the binary carries no build information.
func Version() string {
	if stamped != "" {
		return stamped
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return version(info)
}

func version(info *debug.BuildInfo) string {
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value[:min(7, len(s.Value))]
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	switch {
	case rev == "" && info.Main.Version != "(devel)":
		return info.Main.Version
	case dirty:
		return rev + "-dirty"
	}
	return rev
}
