package camui

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sj14/darts-o-mat/internal/cam/camapp"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

// calibrationFixture is an app with one camera without a device, a calibration in progress on
// the development board's first camera, and the pixel of a click in the double of the 20.
func calibrationFixture(t *testing.T) (ui *UI, app *camapp.App, srv *httptest.Server, doubleTwenty dartboard.Point) {
	t.Helper()
	dir := t.TempDir()
	cfg, err := json.Marshal(camapp.Config{
		Boards: []camapp.BoardConfig{{
			Board:   camapp.Board{ID: 1, Name: "Board", Width: 1280, Height: 720, FPS: 10, Detection: vision.DefaultWatchOptions},
			Detect:  true,
			Cameras: []vision.CameraCalibration{{Name: "cam1", DeviceName: "none", Points: []vision.CalibrationPoint{}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if app, err = camapp.New(dir, camapp.Options{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	ui = New(app)
	srv = httptest.NewServer(ui.Routes())
	t.Cleanup(srv.Close)

	calib, err := vision.LoadCalibration("../vision/testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	cam, _ := calib.Camera("cam1")
	truth, _ := cam.ToBoard()
	toImage, _ := truth.Inverse()
	ui.session["cam1"] = &session{frame: loadRGBA(t, "../vision/testdata/cam1-empty.jpg")}
	return ui, app, srv, toImage.Apply(dartboard.Polar(166, 3))
}

// post sends a form and does not follow the redirect, to see where it leads.
func post(t *testing.T, srv *httptest.Server, path string, form url.Values) *http.Response {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.PostForm(srv.URL+path, form)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

func click(p dartboard.Point) url.Values {
	return url.Values{"click.x": {strconv.FormatFloat(p.X, 'f', 1, 64)}, "click.y": {strconv.FormatFloat(p.Y, 'f', 1, 64)}}
}

// One click on the double of the 20 calibrates the camera, and the result can be refined
// again from its points like a calibration clicked by hand.
func TestCalibrateByOneClick(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long; runs without -short, e.g. in CI")
	}
	ui, app, srv, tap := calibrationFixture(t)

	_, body := get(t, srv, "/cam/cam1/calibrate")
	if !strings.Contains(body, "Click once into the narrow outer ring") || strings.Contains(body, "Click where the wire between") {
		t.Fatalf("the page doesn't ask for the one click first:\n%s", body)
	}

	resp := post(t, srv, "/cam/cam1/auto", click(tap))
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/cam/cam1/check" {
		t.Fatalf("answer %d to %q, want a redirect to the check of the grid", resp.StatusCode, resp.Header.Get("Location"))
	}
	cam, _ := app.Camera("cam1")
	if cam.Refined == nil || cam.Width != 1280 || cam.Height != 720 {
		t.Errorf("not saved: refined %v, size %d×%d", cam.Refined, cam.Width, cam.Height)
	}
	if len(cam.Points) < MinPoints {
		t.Errorf("%d points saved, want at least %d so that it can be refined again", len(cam.Points), MinPoints)
	}
	ui.mu.Lock()
	_, running := ui.session["cam1"]
	ui.mu.Unlock()
	if running {
		t.Error("the calibration in progress is still there")
	}
}

// If the click leads nowhere, the user is told and gets the wire crossings to click.
func TestCalibrateFallsBackToClicking(t *testing.T) {
	t.Parallel()
	ui, app, srv, _ := calibrationFixture(t)

	resp := post(t, srv, "/cam/cam1/auto", click(dartboard.Point{X: 30, Y: 30}))
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/cam/cam1/calibrate" {
		t.Fatalf("answer %d to %q, want back to the calibration", resp.StatusCode, resp.Header.Get("Location"))
	}
	if cam, _ := app.Camera("cam1"); cam.Refined != nil {
		t.Error("something was saved although the board wasn't found")
	}
	_, body := get(t, srv, "/cam/cam1/calibrate")
	if !strings.Contains(body, "wasn&#39;t found automatically") || !strings.Contains(body, "Click where the wire between") {
		t.Errorf("no message and no wire crossings to click:\n%s", body)
	}

	// Now a click on the image is a point for the wire crossing, not another try.
	post(t, srv, "/cam/cam1/auto", click(dartboard.Point{X: 640, Y: 300}))
	ui.mu.Lock()
	manual, points := ui.session["cam1"].manual, len(ui.session["cam1"].points)
	ui.mu.Unlock()
	if !manual || points != 0 {
		t.Errorf("manual %v with %d points after a click for the automatic one", manual, points)
	}
}

// The user can also choose the wire crossings right away.
func TestCalibrateByHandOnRequest(t *testing.T) {
	t.Parallel()
	_, _, srv, _ := calibrationFixture(t)
	post(t, srv, "/cam/cam1/manual", nil)
	_, body := get(t, srv, "/cam/cam1/calibrate")
	if !strings.Contains(body, "Click where the wire between") || strings.Contains(body, "Click once into the narrow outer ring") {
		t.Errorf("the wire crossings aren't offered:\n%s", body)
	}
	if strings.Contains(body, "wasn&#39;t found automatically") {
		t.Error("an error is shown although nothing failed")
	}
}

// A failed action on a board, e.g. refining a calibration again, is shown on the board's
// overview instead of only in the log: the user may have no terminal.
func TestFailureShownOnOverview(t *testing.T) {
	t.Parallel()
	ui, _, _, _ := calibrationFixture(t)
	failed := refineError{errors.New("too few features")}

	w := httptest.NewRecorder()
	ui.doneOnBoard(w, httptest.NewRequest("POST", "/cam/cam1/refine", nil), 1, failed)
	if want := html.EscapeString(textsEN{}.ErrRefine(failed.err)); !strings.Contains(w.Body.String(), want) {
		t.Errorf("the overview doesn't say %q:\n%s", want, w.Body.String())
	}

	w = httptest.NewRecorder()
	ui.doneOnBoard(w, httptest.NewRequest("POST", "/board/1/reconnect", nil), 1, nil)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/board/1" {
		t.Errorf("success: %d to %q, want a redirect to the overview", w.Code, w.Header().Get("Location"))
	}
}
