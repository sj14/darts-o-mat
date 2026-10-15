package main

import (
	"context"
	"html"
	"log"
	"runtime"

	"github.com/crgimenes/glaze"
	"github.com/crgimenes/glaze/menu"
)

// The main goroutine stays on the process's main thread: macOS only shows windows
// there (see showWindow).
func init() { runtime.LockOSThread() }

// showWindow shows the web interface in a window of its own until the user closes it
// or ctx is done. It uses the web view of the system, loaded at runtime without cgo
// (WKWebView on macOS, WebKitGTK on Linux, WebView2 on Windows), so darts-o-cam also
// starts on computers without it, e.g. a Raspberry Pi without a desktop: then it
// returns an error. It must be called on the main goroutine.
func showWindow(ctx context.Context, url string) error {
	w, err := glaze.NewWithOptions(glaze.Options{NoBridge: true}) // the page needs no Go functions
	if err != nil {
		return err
	}
	defer w.Destroy()
	w.SetTitle("darts-o-cam")
	w.SetSize(1280, 900, glaze.HintNone)
	setMenu(w)
	w.Navigate(url)
	defer context.AfterFunc(ctx, w.Terminate)()
	w.Run()
	return nil
}

// showError shows why darts-o-cam can't start in a small window, if there is a web
// view: started as an app, there is no terminal to read it in.
func showError(err error) {
	w, werr := glaze.NewWithOptions(glaze.Options{NoBridge: true})
	if werr != nil {
		return
	}
	defer w.Destroy()
	w.SetTitle("darts-o-cam")
	w.SetSize(520, 220, glaze.HintNone)
	setMenu(w)
	w.SetHtml(`<body style="margin:0;padding:24px;background:#0f1216;color:#f4e6ca;font:16px -apple-system,system-ui,sans-serif">` +
		`<h2 style="margin-top:0;color:#db2b33">darts-o-cam could not start</h2><p>` + html.EscapeString(err.Error()) + `</p></body>`)
	w.Run()
}

// setMenu gives the app the menu bar of a macOS app: quit and the edit commands. Without
// an Edit menu, macOS has no route for Cmd+V, C, X, A, Z to the web view: pasting the
// camera token only beeped (2026-10-15). Windows and Linux handle these keys in the web
// view themselves and get no menu. Must be called before w.Run, on the main goroutine.
func setMenu(w glaze.WebView) {
	if runtime.GOOS != "darwin" {
		return
	}
	_, err := menu.Set([]menu.Item{
		{Title: "darts-o-cam", Submenu: []menu.Item{
			{Title: "Quit darts-o-cam", Shortcut: "cmd+q", OnClick: w.Terminate},
		}},
		{Title: "Edit", Submenu: []menu.Item{
			{Title: "Undo", Shortcut: "cmd+z", Selector: "undo:"},
			{Title: "Redo", Shortcut: "cmd+shift+z", Selector: "redo:"},
			{Separator: true},
			{Title: "Cut", Shortcut: "cmd+x", Selector: "cut:"},
			{Title: "Copy", Shortcut: "cmd+c", Selector: "copy:"},
			{Title: "Paste", Shortcut: "cmd+v", Selector: "paste:"},
			{Title: "Select All", Shortcut: "cmd+a", Selector: "selectAll:"},
		}},
	}, menu.Options{})
	if err != nil {
		log.Printf("menu: %v", err)
	}
}
