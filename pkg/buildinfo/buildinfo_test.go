package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestVersion(t *testing.T) {
	settings := func(rev, modified string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: modified}}
	}
	tests := []struct {
		name string
		info debug.BuildInfo
		want string
	}{
		{"revision", debug.BuildInfo{Settings: settings("0123456789abcdef", "false")}, "0123456"},
		{"dirty", debug.BuildInfo{Settings: settings("0123456789abcdef", "true")}, "0123456-dirty"},
		{"module version", debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}}, "v1.2.3"},
		{"nothing", debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, ""},
	}
	for _, tt := range tests {
		if got := version(&tt.info); got != tt.want {
			t.Errorf("%s: version = %q, want %q", tt.name, got, tt.want)
		}
	}
}
