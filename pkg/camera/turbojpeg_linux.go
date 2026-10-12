//go:build !noturbojpeg

package camera

// libjpeg-turbo decodes MJPEG with SIMD, straight to RGBA (measured at 1280×720:
// image/jpeg 11.5 ms plus 5.6 ms to convert to RGBA). It is optional: it is loaded at
// runtime through purego if it is installed, otherwise frames are decoded with image/jpeg.
// It uses the TurboJPEG 2 API, which libjpeg-turbo 3 still offers.
//
// purego makes even a build with CGO_ENABLED=0 depend on the glibc loader; the tag
// noturbojpeg leaves libjpeg-turbo out for a static binary (e.g. for musl).

import (
	"image"
	"sync"

	"github.com/ebitengine/purego"
)

var (
	turboOnce sync.Once
	turboErr  error

	tjInitDecompress    func() uintptr
	tjDecompressHeader3 func(handle uintptr, jpeg *byte, size uintptr, width, height, subsamp, colorspace *int32) int32
	tjDecompress2       func(handle uintptr, jpeg *byte, size uintptr, dst *byte, width, pitch, height, pixelFormat, flags int32) int32
	tjGetErrorCode      func(handle uintptr) int32
	tjDestroy           func(handle uintptr) int32
)

const (
	tjPixelFormatRGBA = 7 // TJPF_RGBA
	tjErrorFatal      = 1 // TJERR_FATAL; warnings (e.g. extra bytes in the frame) still decode it
)

func loadTurbo() error {
	turboOnce.Do(func() {
		lib, err := purego.Dlopen("libturbojpeg.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			turboErr = err
			return
		}
		// RegisterLibFunc panics on a missing function, so look them all up first.
		for _, f := range []struct {
			fn   any
			name string
		}{
			{&tjInitDecompress, "tjInitDecompress"},
			{&tjDecompressHeader3, "tjDecompressHeader3"},
			{&tjDecompress2, "tjDecompress2"},
			{&tjGetErrorCode, "tjGetErrorCode"},
			{&tjDestroy, "tjDestroy"},
		} {
			addr, err := purego.Dlsym(lib, f.name)
			if err != nil {
				turboErr = err
				return
			}
			purego.RegisterFunc(f.fn, addr)
		}
	})
	return turboErr
}

// turboDecoder is a TurboJPEG handle, 0 if libjpeg-turbo isn't available. A handle
// must not be used concurrently, so each camera has its own.
type turboDecoder uintptr

func newTurboDecoder() turboDecoder {
	if loadTurbo() != nil {
		return 0
	}
	return turboDecoder(tjInitDecompress())
}

// decode returns nil for a broken frame.
func (t turboDecoder) decode(data []byte) *image.RGBA {
	if len(data) == 0 {
		return nil
	}
	var w, h, subsamp, colorspace int32
	if tjDecompressHeader3(uintptr(t), &data[0], uintptr(len(data)), &w, &h, &subsamp, &colorspace) != 0 || w <= 0 || h <= 0 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	if tjDecompress2(uintptr(t), &data[0], uintptr(len(data)), &img.Pix[0], w, int32(img.Stride), h, tjPixelFormatRGBA, 0) != 0 &&
		tjGetErrorCode(uintptr(t)) == tjErrorFatal {
		return nil
	}
	return img
}

func (t turboDecoder) close() {
	if t != 0 {
		tjDestroy(uintptr(t))
	}
}
