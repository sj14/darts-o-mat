//go:build noturbojpeg

package camera

import "image"

// turboBuilt is false: this build leaves libjpeg-turbo out on purpose.
const turboBuilt = false

// turboDecoder is always 0 without libjpeg-turbo: frames are decoded with image/jpeg.
type turboDecoder uintptr

func newTurboDecoder() turboDecoder            { return 0 }
func (turboDecoder) decode([]byte) *image.RGBA { return nil }
func (turboDecoder) close()                    {}
