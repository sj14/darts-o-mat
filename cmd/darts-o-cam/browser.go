package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// hasDisplay reports whether darts-o-cam runs on a desktop, where it opens the browser,
// rather than headless (e.g. a Raspberry Pi next to the board, used from another device).
func hasDisplay() bool {
	if os.Getenv("SSH_CONNECTION") != "" {
		return false // the display, if any, is someone else's
	}
	switch runtime.GOOS {
	case "darwin", "windows":
		return true
	}
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

// reveal shows a file in the file manager (Finder, or the folder it is in).
func reveal(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,"+path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	return cmd.Start()
}

// openBrowser opens url in the default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
