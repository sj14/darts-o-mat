package vision

import (
	"cmp"
	"errors"
	"image"
	"sync"
	"time"

	"github.com/sj14/darts-o-mat/pkg/camera"
)

// settleTime lets the cameras adjust their exposure after opening, before their
// views are compared and the exposure is locked.
const settleTime = time.Second

// ErrNotConnected means that no device was found for a camera of a calibration.
var ErrNotConnected = errors.New("not connected")

// OpenCameras opens the cameras of a calibration, in the calibration's order. Each camera
// is found among the devices with its DeviceName by comparing their views with the
// reference images, so it doesn't matter which USB port it is plugged into. A camera
// that isn't connected stays nil, with ErrNotConnected in problems. If the views of a
// device name don't match the references (e.g. a camera was moved), those cameras are
// opened in device order with ErrAmbiguousCameras in problems: they need to be
// calibrated again. The exposure is locked once it settled.
func OpenCameras(calib Calibration, opts camera.Options) (cams []*camera.Camera, problems []error) {
	cams = make([]*camera.Camera, len(calib.Cameras))
	problems = make([]error, len(calib.Cameras))
	if len(calib.Cameras) == 0 {
		return cams, problems
	}
	devices, err := camera.Devices()
	if err != nil {
		for i := range problems {
			problems[i] = err
		}
		return cams, problems
	}

	// The cameras of each device name, in the calibration's order.
	var names []string
	byName := map[string][]int{}
	for i, c := range calib.Cameras {
		if _, ok := byName[c.DeviceName]; !ok {
			names = append(names, c.DeviceName)
		}
		byName[c.DeviceName] = append(byName[c.DeviceName], i)
	}
	for _, name := range names {
		openGroup(calib, byName[name], devices, opts, cams, problems)
	}
	return cams, problems
}

// openGroup opens the cameras of a calibration that share a device name (indexes),
// see OpenCameras.
func openGroup(calib Calibration, indexes []int, devices []camera.Device, opts camera.Options, cams []*camera.Camera, problems []error) {
	fail := func(err error) {
		for _, i := range indexes {
			problems[i] = err
		}
	}
	var candidates []*camera.Camera
	var openErr error
	for _, d := range devices {
		if d.Name != calib.Cameras[indexes[0]].DeviceName {
			continue
		}
		cam, err := camera.Open(d.ID, opts)
		if err != nil {
			openErr = err // e.g. used by another program
			continue
		}
		candidates = append(candidates, cam)
	}
	if len(candidates) == 0 {
		fail(cmp.Or(openErr, ErrNotConnected))
		return
	}
	frames, err := settle(candidates)
	if err != nil {
		closeAll(candidates)
		fail(err)
		return
	}

	var views, refs []image.Image
	for _, f := range frames {
		views = append(views, f)
	}
	for _, i := range indexes {
		ref, err := calib.LoadReference(calib.Cameras[i])
		if err != nil {
			closeAll(candidates)
			fail(err)
			return
		}
		refs = append(refs, ref)
	}
	assignment, err := AssignCameras(views, refs)
	var mismatch error
	if errors.Is(err, ErrAmbiguousCameras) {
		// Open them anyway, in device order, so they can be calibrated again.
		mismatch = err
		assignment = make([]int, len(refs))
		for r := range assignment {
			assignment[r] = -1
			if r < len(candidates) {
				assignment[r] = r
			}
		}
	} else if err != nil {
		closeAll(candidates)
		fail(err)
		return
	}

	used := make([]bool, len(candidates))
	for r, f := range assignment {
		i := indexes[r]
		if f < 0 {
			problems[i] = ErrNotConnected
			continue
		}
		cams[i], problems[i], used[f] = candidates[f], mismatch, true
	}
	for f, c := range candidates {
		if !used[f] {
			c.Close()
		}
	}
}

// OpenCamera opens a single device, e.g. a camera being added to a board, and returns
// it with a frame taken once the exposure settled; the exposure is locked then.
func OpenCamera(id string, opts camera.Options) (*camera.Camera, *image.RGBA, error) {
	cam, err := camera.Open(id, opts)
	if err != nil {
		return nil, nil, err
	}
	frames, err := settle([]*camera.Camera{cam})
	if err != nil {
		cam.Close()
		return nil, nil, err
	}
	return cam, frames[0], nil
}

func closeAll(cams []*camera.Camera) {
	for _, c := range cams {
		c.Close()
	}
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

// Format tells how the camera's frames arrive, see camera.Camera.Format.
func (l *Live) Format() string {
	return l.cam.Format
}

func (l *Live) Close() error {
	return l.cam.Close()
}
