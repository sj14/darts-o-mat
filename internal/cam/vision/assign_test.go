package vision

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"slices"
	"testing"

	"github.com/sj14/darts-o-mat/internal/common/dartboard"
)

func TestAssignCameras(t *testing.T) {
	cam1, cam2, cam3 := loadRGBA(t, "testdata/cam1.jpg"), loadRGBA(t, "testdata/cam2.jpg"), loadRGBA(t, "testdata/cam3.jpg")
	refs := []image.Image{cam1, cam2, cam3}

	// The devices come up in a different order, with a dart added in one view.
	withDart := loadRGBA(t, "testdata/cam1.jpg")
	drawDart(withDart, dartboardPoint(700, 400), dartboardPoint(0.2, -1))
	got, err := AssignCameras([]image.Image{cam3, withDart, cam2}, refs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 0}; !slices.Equal(got, want) {
		t.Errorf("assignment = %v, want %v", got, want)
	}

	// A laptop camera instead of one of the board's cameras.
	other := image.NewRGBA(cam1.Bounds())
	draw.Draw(other, other.Bounds(), &image.Uniform{color.RGBA{200, 180, 150, 255}}, image.Point{}, draw.Src)
	if _, err := AssignCameras([]image.Image{cam1, other, cam3}, refs); !errors.Is(err, ErrAmbiguousCameras) {
		t.Errorf("foreign view: got %v, want ErrAmbiguousCameras", err)
	}

	// A camera is missing: the others are still found.
	got, err = AssignCameras([]image.Image{cam3, cam1}, refs)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, -1, 0}; !slices.Equal(got, want) {
		t.Errorf("assignment with a camera missing = %v, want %v", got, want)
	}
}

func dartboardPoint(x, y float64) dartboard.Point {
	return dartboard.Point{X: x, Y: y}
}
