package camui

import (
	"errors"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/pkg/camapp"
	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

func loadRGBA(t *testing.T, path string) *image.RGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return vision.ToRGBA(img)
}

// newUI serves an app without cameras, with its configuration in a temporary directory.
func newUI(t *testing.T) (*camapp.App, *httptest.Server) {
	t.Helper()
	app, err := camapp.New(t.TempDir(), camapp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	srv := httptest.NewServer(New(app).Routes())
	t.Cleanup(srv.Close)
	return app, srv
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestEventPages(t *testing.T) {
	app, srv := newUI(t)
	frames := []*image.RGBA{
		loadRGBA(t, "../vision/testdata/cam1.jpg"),
		loadRGBA(t, "../vision/testdata/cam2.jpg"),
		loadRGBA(t, "../vision/testdata/cam3.jpg"),
	}
	names := []string{"cam1", "cam2", "cam3"}
	app.AddEvent(camapp.Event{Cameras: names, Game: 12, Event: &vision.Event{
		Kind:    vision.EventDart,
		Board:   dartboard.Point{X: 71, Y: 67},
		Dart:    throw.Dart{Number: 4, Multiplier: 1},
		Cameras: map[int]dartboard.Point{0: {X: 74, Y: 66}, 1: {X: 71, Y: 69}, 2: {X: 69, Y: 67}},
		Tips:    map[int]dartboard.Point{0: {X: 826, Y: 342}, 1: {X: 495, Y: 249}, 2: {X: 765, Y: 228}},
		Before:  frames,
		After:   frames,
	}})
	app.AddEvent(camapp.Event{Cameras: names, ReportErr: errors.New("no game"), Event: &vision.Event{Kind: vision.EventTakeout, After: frames}})

	code, body := get(t, srv, "/")
	if code != 200 || !strings.Contains(body, ">4<") || !strings.Contains(body, "herausgezogen") {
		t.Errorf("overview: %d, events missing", code)
	}
	if !strings.Contains(body, "Spiel 12") || !strings.Contains(body, "no game") {
		t.Errorf("overview: what the server did is missing")
	}
	if !strings.Contains(body, "wartet auf eine kalibrierte Kamera") {
		t.Errorf("overview: detection status missing")
	}
	code, body = get(t, srv, "/event/1")
	if code != 200 || !strings.Contains(body, "<svg") || !strings.Contains(body, "97.6") {
		t.Errorf("event page: %d, sketch or distance missing", code)
	}
	if code, body := get(t, srv, "/event/1/cam/0.jpg"); code != 200 || !strings.HasPrefix(body, "\xff\xd8") {
		t.Errorf("event image: %d, not a JPEG", code)
	}
	if !strings.Contains(body, `href="/event/1/cam/2/before.jpg"`) || !strings.Contains(body, `href="/event/1/cam/2/after.jpg"`) {
		t.Errorf("event page: links to the whole frames missing")
	}

	// The whole frames download under the names darts-o-cam -save uses.
	resp, err := http.Get(srv.URL + "/event/1/cam/2/before.jpg")
	if err != nil {
		t.Fatal(err)
	}
	frame, _, err := image.Decode(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("whole frame: %v", err)
	}
	if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename="001-cam3-before.jpg"` {
		t.Errorf("whole frame: Content-Disposition %q", got)
	}
	if frame.Bounds().Dx() != 1280 {
		t.Errorf("whole frame is %v, want the camera's 1280×720", frame.Bounds())
	}
	for _, path := range []string{"/event/1/cam/3/before.jpg", "/event/1/cam/0/during.jpg", "/event/99/cam/0/after.jpg"} {
		if code, _ := get(t, srv, path); code != 404 {
			t.Errorf("%s: %d, want 404", path, code)
		}
	}

	if code, _ := get(t, srv, "/event/99"); code != 404 {
		t.Errorf("unknown event: %d, want 404", code)
	}
	if code, _ := get(t, srv, "/cam/cam9/calibrate"); code != 404 {
		t.Errorf("unknown camera: %d, want 404", code)
	}
}

func TestSettings(t *testing.T) {
	app, srv := newUI(t)
	post := func(values url.Values) (int, string) {
		t.Helper()
		// The detection's thresholds as the form shows them, unless given.
		for _, f := range detectionFields(vision.DefaultWatchOptions) {
			if !values.Has(f.Name) {
				values.Set(f.Name, f.Value)
			}
		}
		resp, err := http.PostForm(srv.URL+"/settings", values)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	code, body := post(url.Values{"server": {"http://darts.local:3000/"}, "token": {" abc "}, "size": {"640x480"}, "fps": {"15"}})
	if code != 200 || !strings.Contains(body, "Gespeichert") {
		t.Errorf("save: %d, not saved", code)
	}
	want := camapp.Settings{Server: "http://darts.local:3000", Token: "abc", Width: 640, Height: 480, FPS: 15, Detection: app.Settings().Detection}
	if got := app.Settings(); got != want {
		t.Errorf("settings = %+v, want %+v", got, want)
	}
	if blur := want.Detection.Detect.Blur; blur != 1.42 {
		t.Errorf("blur = %v mm, want 1.42 as shown in the form", blur)
	}
	// Enter in a field submits with the form's first own submit button (buttons with a
	// form attribute belong to another form): that must save.
	_, page := get(t, srv, "/settings")
	form := page[strings.Index(page, `<form method="post" action="/settings"`):]
	form = form[:strings.Index(form, "</form>")]
	for _, button := range strings.Split(form, "<button")[1:] {
		tag := button[:strings.Index(button, ">")]
		if !strings.Contains(tag, `type="submit"`) || strings.Contains(tag, "form=") {
			continue
		}
		if !strings.HasPrefix(button[len(tag)+1:], "Speichern") {
			t.Errorf("the settings form's first submit button isn't Speichern: %s", button[:min(len(button), 300)])
		}
		break
	}

	if _, body := get(t, srv, "/"); !strings.Contains(body, "http://darts.local:3000") {
		t.Errorf("overview doesn't show the server")
	}

	if _, body := post(url.Values{"server": {"darts.local"}, "size": {"640x480"}, "fps": {"15"}}); !strings.Contains(body, "Ungültige Einstellungen") {
		t.Errorf("server without scheme: no error shown")
	}
	if _, body := post(url.Values{"size": {"640x480"}, "fps": {"15"}, "weak_threshold": {"30"}}); !strings.Contains(body, "Ungültige Einstellungen") {
		t.Errorf("weak threshold above the strong one: no error shown")
	}
	if got := app.Settings(); got != want {
		t.Errorf("invalid settings were stored: %+v", got)
	}

	resp, err := http.PostForm(srv.URL+"/settings/defaults", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got := app.Settings().Detection; got != vision.DefaultWatchOptions {
		t.Errorf("after resetting: detection %+v, want the defaults", got)
	}
	want.Detection = vision.DefaultWatchOptions

	// The settings are kept for the next start.
	again, err := camapp.New(app.Dir(), camapp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := again.Settings(); got != want {
		t.Errorf("settings after a restart = %+v, want %+v", got, want)
	}
}

// The configuration directory doesn't exist before the first start: the cameras'
// images are saved into it when a camera is added, before any configuration is.
func TestNewCreatesDir(t *testing.T) {
	dir := t.TempDir() + "/darts-o-cam"
	app, err := camapp.New(dir, camapp.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("config directory not created: %v", err)
	}
}
