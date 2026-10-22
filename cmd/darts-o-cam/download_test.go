package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUniquePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := uniquePath(dir, "028-cam3-before.jpg"), filepath.Join(dir, "028-cam3-before.jpg"); got != want {
		t.Errorf("free name: %q, want %q", got, want)
	}
	touch("028-cam3-before.jpg")
	if got, want := uniquePath(dir, "028-cam3-before.jpg"), filepath.Join(dir, "028-cam3-before (1).jpg"); got != want {
		t.Errorf("taken name: %q, want %q", got, want)
	}
	touch("028-cam3-before (1).jpg")
	if got, want := uniquePath(dir, "028-cam3-before.jpg"), filepath.Join(dir, "028-cam3-before (2).jpg"); got != want {
		t.Errorf("twice taken: %q, want %q", got, want)
	}
	touch("noext")
	if got, want := uniquePath(dir, "noext"), filepath.Join(dir, "noext (1)"); got != want {
		t.Errorf("without extension: %q, want %q", got, want)
	}
}

// A page can only suggest a name; it never reaches a folder of its own choosing.
func TestCleanName(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"001-cam1-before.jpg":    "001-cam1-before.jpg",
		"../../.ssh/config":      "config",
		`..\..\Windows\evil.exe`: "evil.exe",
		"/etc/passwd":            "passwd",
		"":                       "download",
		"..":                     "download",
		"/":                      "download",
	} {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
