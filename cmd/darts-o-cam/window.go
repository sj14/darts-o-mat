package main

import (
	"context"
	"html"
	"log"
	"runtime"
	"strings"

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
	w, err := glaze.NewWithOptions(windowOptions())
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

// windowOptions are what the window does beyond showing the page, where a browser would do
// it for the user: it has no address bar, no back button and no download manager.
func windowOptions() glaze.Options {
	return glaze.Options{
		NoBridge: true, // the page needs no Go functions
		// A link with target=_blank (the project page, the new version, a whole image) never
		// opens a window in glaze, it only calls this: without it the link does nothing.
		OnNewWindow: openExternal,
		// Without OnDownload every download is cancelled (the frames of an event). The
		// window has no save panel, so the file goes into the Downloads folder under a name
		// that isn't taken, and is shown there (macOS and Linux; Windows handles downloads
		// itself).
		OnDownload: downloadPath,
		OnDownloadDone: func(path string, err error) {
			if err != nil {
				log.Printf("download %s: %v", path, err)
				return
			}
			if err := reveal(path); err != nil {
				log.Printf("show %s: %v", path, err)
			}
		},
		// Two fingers sideways go back and forward, as in Safari (macOS and Linux).
		NavigationGestures: true,
		// The window comes back where and as big as it was (macOS).
		FrameAutosaveName: "darts-o-cam",
	}
}

// openExternal opens what a link asked a new window for in the browser, if it is a web page.
func openExternal(url string) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return
	}
	if err := openBrowser(url); err != nil {
		log.Printf("open %s: %v", url, err)
	}
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

// viewMenu is what a browser's toolbar would be: back and forward through the pages (there is
// no back button in the window), reload and zoom, e.g. to see the whole camera image when
// calibrating on a small screen.
func viewMenu(w glaze.WebView) []menu.Item {
	zoom := 1.0
	zoomTo := func(factor float64) func() {
		return func() {
			zoom = min(3, max(0.5, factor))
			w.SetZoom(zoom)
		}
	}
	step := func(by float64) func() { return func() { zoomTo(zoom + by)() } }
	return []menu.Item{
		{Title: "Back", Shortcut: "cmd+[", OnClick: w.GoBack},
		{Title: "Forward", Shortcut: "cmd+]", OnClick: w.GoForward},
		{Title: "Reload", Shortcut: "cmd+r", OnClick: w.Reload},
		{Separator: true},
		{Title: "Zoom In", Shortcut: "cmd+=", OnClick: step(0.1)},
		{Title: "Zoom Out", Shortcut: "cmd+-", OnClick: step(-0.1)},
		{Title: "Actual Size", Shortcut: "cmd+0", OnClick: zoomTo(1)},
	}
}

// setMenu gives the app the menu bar of a macOS app: quit, view and the edit commands. Without
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
		{Title: "View", Submenu: viewMenu(w)},
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
