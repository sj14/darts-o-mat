package camapp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/cam/vision"
)

func TestBoards(t *testing.T) {
	dir := t.TempDir()
	cfg := defaultConfig()
	cfg.Boards = append(cfg.Boards, newBoard(2, "Garage"))
	cfg.Boards[1].Cameras = []vision.CameraCalibration{{Name: "cam1", DeviceName: "none"}}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, configFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
	a, err := New(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	if got := len(a.Cameras(1)); got != 0 {
		t.Errorf("board 1 has %d cameras, want none", got)
	}
	if c, ok := a.Camera("cam1"); !ok || c.BoardID != 2 {
		t.Errorf("cam1 = %+v, %v, want it at board 2", c, ok)
	}

	// A board with cameras stays until they're removed.
	if err := a.RemoveBoard(2); !errors.Is(err, ErrBoardHasCameras) {
		t.Errorf("remove board with cameras: %v", err)
	}
	if err := a.RemoveCamera("cam1"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveBoard(2); err != nil {
		t.Errorf("remove board: %v", err)
	}
	if err := a.RemoveBoard(1); !errors.Is(err, ErrLastBoard) {
		t.Errorf("remove the last board: %v", err)
	}

	// New boards get free IDs and names, even if a board was renamed to the next name.
	first := a.Boards()[0]
	first.Name = "Board 2"
	if err := a.SetBoard(first); err != nil {
		t.Fatal(err)
	}
	id, err := a.AddBoard()
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Errorf("new board has ID %d, want 2", id)
	}
	names := []string{}
	for _, b := range a.Boards() {
		names = append(names, b.Name)
	}
	if !slices.Equal(names, []string{"Board 2", "Board 3"}) {
		t.Errorf("names %v, want Board 2 and Board 3", names)
	}

	// Names are needed and unique.
	second := a.Boards()[1]
	second.Name = "Board 2"
	if err := a.SetBoard(second); !errors.Is(err, ErrInvalidSettings) {
		t.Errorf("same name twice: %v", err)
	}
	second.Name = " "
	if err := a.SetBoard(second); !errors.Is(err, ErrInvalidSettings) {
		t.Errorf("no name: %v", err)
	}
}

// Detections are kept per board: each shows its own.
func TestEventsPerBoard(t *testing.T) {
	a, err := New(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	e1, e2 := event(), event()
	e1.BoardID, e2.BoardID = 1, 2
	id1, id2 := a.AddEvent(e1), a.AddEvent(e2)
	if got := a.Events(1); len(got) != 1 || got[0].ID != id1 {
		t.Errorf("board 1: %v", got)
	}
	if got := a.Events(2); len(got) != 1 || got[0].ID != id2 {
		t.Errorf("board 2: %v", got)
	}
}

// Each board has settings of its own.
func TestBoardSettingsAreSeparate(t *testing.T) {
	a, err := New(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	id, err := a.AddBoard()
	if err != nil {
		t.Fatal(err)
	}
	second := a.Boards()[1]
	second.Width, second.Height, second.FPS = 640, 480, 15
	second.Detection.Detect.DiffThreshold++
	if err := a.SetBoard(second); err != nil {
		t.Fatal(err)
	}
	first := a.Boards()[0]
	if first.Width != 1280 || first.FPS != 10 || first.Detection == second.Detection {
		t.Errorf("board 1 changed with board %d: %+v", id, first)
	}
	if got := a.Boards()[1]; got != second {
		t.Errorf("board 2 = %+v, want %+v", got, second)
	}
}

func TestSmallFrames(t *testing.T) {
	b := newBoard(1, "Board") // wants 1280×720
	for _, tc := range []struct {
		width, height int
		small         bool
	}{
		{0, 0, false}, // not open
		{160, 120, true},
		{320, 240, true},
		{640, 480, false},
		{1280, 720, false},
	} {
		a := &App{cams: map[string]*boardCamera{"cam1": {width: tc.width, height: tc.height}}}
		if got := a.state(&b, vision.CameraCalibration{Name: "cam1"}).Small; got != tc.small {
			t.Errorf("%d×%d: small %v, want %v", tc.width, tc.height, got, tc.small)
		}
	}
}
