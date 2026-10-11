package camui

import (
	"image"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

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

func TestEventPages(t *testing.T) {
	calib, err := vision.LoadCalibration("../vision/testdata/calibration.json")
	if err != nil {
		t.Fatal(err)
	}
	ui := New("calibration.json", calib, nil)
	frames := []*image.RGBA{
		loadRGBA(t, "../vision/testdata/cam1.jpg"),
		loadRGBA(t, "../vision/testdata/cam2.jpg"),
		loadRGBA(t, "../vision/testdata/cam3.jpg"),
	}
	ui.AddEvent(&vision.Event{
		Kind:    vision.EventDart,
		Board:   dartboard.Point{X: 71, Y: 67},
		Dart:    throw.Dart{Number: 4, Multiplier: 1},
		Cameras: map[int]dartboard.Point{0: {X: 74, Y: 66}, 1: {X: 71, Y: 69}, 2: {X: 69, Y: 67}},
		Tips:    map[int]dartboard.Point{0: {X: 826, Y: 342}, 1: {X: 495, Y: 249}, 2: {X: 765, Y: 228}},
		After:   frames,
	})
	ui.AddEvent(&vision.Event{Kind: vision.EventTakeout, After: frames})

	srv := httptest.NewServer(ui.Routes())
	defer srv.Close()
	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	// The overview needs cameras for their live images; without any it still lists events.
	if code, body := get("/"); code != 200 || !strings.Contains(body, ">4<") || !strings.Contains(body, "herausgezogen") {
		t.Errorf("overview: %d, events missing", code)
	}
	code, body := get("/event/1")
	if code != 200 || !strings.Contains(body, "<svg") || !strings.Contains(body, "97.6") {
		t.Errorf("event page: %d, sketch or distance missing", code)
	}
	if code, body := get("/event/1/cam/0.jpg"); code != 200 || !strings.HasPrefix(body, "\xff\xd8") {
		t.Errorf("event image: %d, not a JPEG", code)
	}
	if code, _ := get("/event/99"); code != 404 {
		t.Errorf("unknown event: %d, want 404", code)
	}
}
