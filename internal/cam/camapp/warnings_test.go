package camapp

import (
	"context"
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/warning"
)

// warningServer is a darts-o-mat that passes on the bodies of the warnings it gets.
func warningServer(t *testing.T) (*server, <-chan string) {
	t.Helper()
	bodies := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/board/warnings" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		bodies <- string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return newServer(srv.URL, "secret"), bodies
}

// newWarningsApp is an App with only what the warnings need.
func newWarningsApp() *App {
	return &App{warningReports: map[int]warningReport{}, warningsChanged: make(chan struct{}, 1)}
}

// Of the warnings that wait for the server, only a board's newest are sent.
func TestWarningsSendsNewest(t *testing.T) {
	t.Parallel()
	s, bodies := warningServer(t)
	a := newWarningsApp()
	missing := warning.CamerasMissing{Cameras: 3}
	a.reportWarnings(1, s, warning.List{missing})
	a.reportWarnings(1, s, warning.List{missing, warning.SmallFrames{Camera: "cam3", Width: 160, Height: 120}})
	a.reportWarnings(2, nil, warning.List{missing}) // no server: nothing to send

	// Only now: what was queued is merged.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.sendWarnings(ctx)
	want := `{"warnings":[{"code":"cameras_missing","details":{"cameras":3,"watching":0}},` +
		`{"code":"small_frames","details":{"camera":"cam3","width":160,"height":120}}]}`
	if got := <-bodies; got != want {
		t.Errorf("sent %s, want %s", got, want)
	}
	select {
	case body := <-bodies:
		t.Errorf("sent %s as well, want only the newest warnings", body)
	case <-time.After(50 * time.Millisecond):
	}
}

// After a takeout the grid of each camera is checked on the empty board: a camera whose
// calibration doesn't fit its view anymore gets a warning.
func TestGridCheck(t *testing.T) {
	t.Parallel()
	calib, err := vision.LoadCalibration("../vision/testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	start, err := calib.Cameras[0].ToBoard()
	if err != nil {
		t.Fatal(err)
	}
	// cam2 looks at the board from elsewhere: cam1's calibration doesn't fit its view.
	frames := []*image.RGBA{loadFrame(t, "cam1-empty.jpg"), loadFrame(t, "cam2-empty.jpg")}

	c := newGridCheck(detection{name: "board", names: []string{"cam1", "cam2"}, starts: []vision.Homography{start, start}})
	if _, ok := c.done(); ok {
		t.Error("done before a check started")
	}
	c.start(frames)
	if !c.running {
		t.Error("no check running after start")
	}
	var grid warning.List
	for {
		var ok bool
		if grid, ok = c.done(); ok {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if want := (warning.List{warning.GridMismatch{Camera: "cam2"}}); !slices.Equal(grid, want) {
		t.Errorf("grid warnings %v, want %v", grid, want)
	}
	if c.running {
		t.Error("still running after done")
	}
}

func loadFrame(t *testing.T, name string) *image.RGBA {
	t.Helper()
	f, err := os.Open("../vision/testdata/" + name)
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
