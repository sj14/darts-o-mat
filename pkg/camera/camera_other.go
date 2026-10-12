//go:build !darwin && !linux

package camera

type platform struct{}

// Devices lists the connected cameras.
func Devices() ([]Device, error) {
	return nil, ErrUnsupported
}

// Open starts delivering frames of the device with the given ID.
func Open(id string, opts Options) (*Camera, error) {
	return nil, ErrUnsupported
}

// LockExposure keeps the current exposure and white balance.
func (c *Camera) LockExposure() error {
	return ErrUnsupported
}

func (c *Camera) stop() error {
	return nil
}
