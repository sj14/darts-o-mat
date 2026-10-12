//go:build noturbojpeg

package camera

import "image"

// turboDecoder is always 0 without libjpeg-turbo: frames are decoded with image/jpeg.
type turboDecoder uintptr

func newTurboDecoder() turboDecoder            { return 0 }
func (turboDecoder) decode([]byte) *image.RGBA { return nil }
func (turboDecoder) close()                    {}
