// Package project finds the project for the tools, which run in the tools module
// (go -C tools run ./…) and build the programs of the project.
package project

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Dir returns the project's directory: the parent of the tools module, which replaces
// the project with .. (see go.mod).
func Dir() (string, error) {
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/sj14/darts-o-mat/tools").Output()
	if err != nil {
		return "", fmt.Errorf("find the project: %w", err)
	}
	return filepath.Dir(strings.TrimSpace(string(dir))), nil
}
