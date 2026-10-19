package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// downloadPath is where a file the page offers for download is saved: the user's Downloads
// folder (else their home, else the temporary folder) under the suggested name, which is
// numbered ("name (1).jpg") if a file of that name exists, so nothing is ever replaced.
// The window has no save panel to ask with, see showWindow.
func downloadPath(suggested string) string {
	return uniquePath(downloadsDir(), cleanName(suggested))
}

func downloadsDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		if dir := filepath.Join(home, "Downloads"); isDir(dir) {
			return dir
		}
		return home
	}
	return os.TempDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// cleanName keeps only the file name of what the page suggests, never a path.
func cleanName(suggested string) string {
	name := filepath.Base(strings.ReplaceAll(suggested, `\`, "/"))
	if name == "." || name == ".." || name == string(filepath.Separator) || name == "/" {
		return "download"
	}
	return name
}

// uniquePath returns dir/name, or the first of dir/name (1), dir/name (2), … that doesn't
// exist yet; the number goes before the extension.
func uniquePath(dir, name string) string {
	path := filepath.Join(dir, name)
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 1; ; n++ {
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			return path
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, n, ext))
	}
}
