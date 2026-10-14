package main

import (
	"log"
	"os"

	"golang.org/x/sys/windows"
)

// The Windows build is a GUI program (-H windowsgui, see tools/winpkg), so a double click
// opens no console window. Started from a terminal, it attaches to that terminal's console
// to log there; output redirected into a file keeps working without it. The terminal
// doesn't wait for a GUI program: its prompt returns right away and the log follows it.
func init() {
	if valid(windows.STD_ERROR_HANDLE) {
		return // a console or a redirection already
	}
	attach := windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")
	const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS
	if ok, _, _ := attach.Call(attachParentProcess); ok == 0 {
		return // started without a terminal
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	if !valid(windows.STD_OUTPUT_HANDLE) {
		os.Stdout = out
	}
	os.Stderr = out
	log.SetOutput(out) // the log package keeps the os.Stderr it started with
}

// valid reports whether the standard handle std leads somewhere.
func valid(std uint32) bool {
	h, err := windows.GetStdHandle(std)
	return err == nil && h != 0 && h != windows.InvalidHandle
}
