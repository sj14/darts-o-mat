package vision

import (
	"fmt"
	"image"
	"slices"
	"sync"
	"time"

	"github.com/sj14/darts-o-mat/pkg/camera"
)

// settleTime lets the cameras adjust their exposure after opening, before their
// views are compared and the exposure is locked.
const settleTime = time.Second

// OpenCameras opens the cameras of a calibration, in the calibration's order.
// Cameras without a fixed device are found among the devices named DeviceName
// by comparing their view with the reference images.
func OpenCameras(calib Calibration, width, height, fps int) ([]*camera.Camera, error) {
	cams := make([]*camera.Camera, len(calib.Cameras))
	closeAll := func() {
		for _, c := range cams {
			if c != nil {
				c.Close()
			}
		}
	}

	var unresolved []int
	for i, c := range calib.Cameras {
		if c.Device == "" {
			unresolved = append(unresolved, i)
			continue
		}
		cam, err := camera.Open(c.Device, camera.Options{Width: width, Height: height, FPS: fps})
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("camera %s: %w", c.Name, err)
		}
		cams[i] = cam
	}
	if len(unresolved) == 0 {
		if _, err := settle(cams); err != nil {
			closeAll()
			return nil, err
		}
		return cams, nil
	}

	candidates, err := OpenCamerasNamed(calib.DeviceName, width, height, fps)
	if err != nil {
		closeAll()
		return nil, err
	}
	closeCandidates := func(keep map[int]bool) {
		for i, c := range candidates {
			if !keep[i] {
				c.Close()
			}
		}
	}
	// The candidates come first, so frames[i] belongs to candidates[i].
	frames, err := settle(append(slices.Clone(candidates), slices.DeleteFunc(slices.Clone(cams), func(c *camera.Camera) bool { return c == nil })...))
	if err != nil {
		closeCandidates(nil)
		closeAll()
		return nil, err
	}

	var refs []image.Image
	for _, i := range unresolved {
		ref, err := calib.LoadReference(calib.Cameras[i])
		if err != nil {
			closeCandidates(nil)
			closeAll()
			return nil, err
		}
		refs = append(refs, ref)
	}
	var views []image.Image
	for _, f := range frames[:len(candidates)] {
		views = append(views, f)
	}
	assignment, err := AssignCameras(views, refs)
	if err != nil {
		closeCandidates(nil)
		closeAll()
		return nil, fmt.Errorf("%d devices named %q: %w", len(candidates), calib.DeviceName, err)
	}

	keep := map[int]bool{}
	for r, f := range assignment {
		cams[unresolved[r]] = candidates[f]
		keep[f] = true
	}
	closeCandidates(keep)
	return cams, nil
}

// OpenCamerasNamed opens all cameras with the given name, in device order. It is used
// to set up a board that has no calibration yet. The exposure isn't locked yet.
func OpenCamerasNamed(name string, width, height, fps int) ([]*camera.Camera, error) {
	devices, err := camera.Devices()
	if err != nil {
		return nil, err
	}
	var cams []*camera.Camera
	for _, d := range devices {
		if d.Name != name {
			continue
		}
		cam, err := camera.Open(d.ID, camera.Options{Width: width, Height: height, FPS: fps})
		if err != nil {
			for _, c := range cams {
				c.Close()
			}
			return nil, err
		}
		cams = append(cams, cam)
	}
	if len(cams) == 0 {
		return nil, fmt.Errorf("no camera named %q", name)
	}
	return cams, nil
}

// settle waits until the cameras have adjusted their exposure, then locks it, so a
// hand in front of a camera doesn't change its whole image (seen when pulling darts
// out). It returns a frame of each camera. Cameras that can't lock keep adjusting.
func settle(cams []*camera.Camera) ([]*image.RGBA, error) {
	time.Sleep(settleTime)
	frames := make([]*image.RGBA, len(cams))
	for i, c := range cams {
		img, err := c.Read()
		if err != nil {
			return nil, err
		}
		frames[i] = img
		_ = c.LockExposure() // without the lock, detection works as before
	}
	return frames, nil
}

// Live reads a camera in the background and keeps its newest frame, so several users
// (detection, calibration interface) can share the camera.
type Live struct {
	cam    *camera.Camera
	mu     sync.Mutex
	cond   *sync.Cond
	latest *image.RGBA
	seq    uint64 // counts the frames read
	err    error
}

// StartLive starts reading the camera in the background.
func StartLive(cam *camera.Camera) *Live {
	l := &Live{cam: cam}
	l.cond = sync.NewCond(&l.mu)
	go func() {
		for {
			img, err := cam.Read()
			l.mu.Lock()
			if err == nil {
				l.latest = img
				l.seq++
			}
			l.err = err
			l.cond.Broadcast()
			l.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return l
}

// Latest returns the newest frame, nil until the first one arrived.
func (l *Live) Latest() (*image.RGBA, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.latest, l.err
}

// Next waits for a frame newer than the one numbered after and returns it with its number.
func (l *Live) Next(after uint64) (*image.RGBA, uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for l.seq <= after && l.err == nil {
		l.cond.Wait()
	}
	return l.latest, l.seq, l.err
}

// Device returns the camera's capture device.
func (l *Live) Device() camera.Device {
	return l.cam.Device
}

func (l *Live) Close() error {
	return l.cam.Close()
}
