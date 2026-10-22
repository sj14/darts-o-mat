//go:build !noturbojpeg

package camera

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"testing"
)

// TestTurboDecode compares libjpeg-turbo with image/jpeg on a frame of the development board.
func TestTurboDecode(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../vision/testdata/cam1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if err := loadTurbo(); err != nil {
		t.Skipf("libjpeg-turbo not available: %v", err)
	}
	turbo := newTurboDecoder()
	defer turbo.close()
	got := turbo.decode(data)

	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(src.Bounds())
	draw.Draw(want, want.Bounds(), src, image.Point{}, draw.Src)
	if got == nil || got.Bounds() != want.Bounds() {
		t.Fatalf("decoded %v, want %v", got, want.Bounds())
	}
	// The decoders round and upsample the colours a little differently.
	var sum, worst int
	for i := range got.Pix {
		d := max(int(got.Pix[i])-int(want.Pix[i]), int(want.Pix[i])-int(got.Pix[i]))
		sum += d
		worst = max(worst, d)
	}
	if mean := float64(sum) / float64(len(got.Pix)); mean > 1 {
		t.Errorf("mean difference %.2f per channel (worst %d), want ≤ 1", mean, worst)
	}

	if turbo.decode(data[:len(data)/10]) == nil {
		t.Log("cut-off frame dropped") // libjpeg-turbo may also fill in the rest with a warning
	}
	if turbo.decode([]byte("no jpeg")) != nil {
		t.Error("decoded garbage")
	}
}

func BenchmarkDecode(b *testing.B) {
	data, err := os.ReadFile("../vision/testdata/cam1.jpg")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("image/jpeg", func(b *testing.B) {
		c := &Camera{platform: platform{format: pixFormat{PixelFormat: pixelFormatMJPEG}}}
		for b.Loop() {
			c.decode(data)
		}
	})
	b.Run("libjpeg-turbo", func(b *testing.B) {
		if err := loadTurbo(); err != nil {
			b.Skip(err)
		}
		c := &Camera{platform: platform{format: pixFormat{PixelFormat: pixelFormatMJPEG}, turbo: newTurboDecoder()}}
		defer c.turbo.close()
		for b.Loop() {
			c.decode(data)
		}
	})
}
