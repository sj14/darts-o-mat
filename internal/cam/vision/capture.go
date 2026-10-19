package vision

import (
	"cmp"
	"errors"
	"image"
	"math"
	"sync"
	"time"

	"github.com/sj14/darts-o-mat/internal/cam/camera"
)

// The cameras adjust their exposure after opening, before their views are compared and
// the exposure is locked (settle): for settleTime at least, then until the brightness
// changes by less than settledChange (of 255) between two frames settleStep apart, for
// maxSettleTime at most. Some cameras need several seconds, e.g. a webcam in a dim room;
// a frame taken before is too dark, and locked so dark it would stay.
const (
	settleTime    = time.Second
	maxSettleTime = 6 * time.Second
	settleStep    = 250 * time.Millisecond
	settledChange = 1.0
)

// ErrNotConnected means that no device was found for a camera of a calibration.
var ErrNotConnected = errors.New("not connected")

// OpenCameras opens the cameras of a calibration, in the calibration's order. Each camera
// is found among the devices with its DeviceName by comparing their views with the
// reference images, so it doesn't matter which USB port it is plugged into. A camera
// that isn't connected stays nil, with ErrNotConnected in problems. If the views of a
// device name don't match the references (e.g. a camera was moved), those cameras are
// opened in device order with ErrAmbiguousCameras in problems: they need to be
// calibrated again. The exposure is locked once it settled.
//
// All cameras of the calibration are assigned together, so identical cameras of several
// boards are told apart as well as those of one. opts says how to open each camera; the
// devices are first opened like the group's first camera and reopened for the cameras
// that want something else. Devices in busy (by ID), e.g. open already for other
// cameras, are left alone.
func OpenCameras(calib Calibration, opts func(CameraCalibration) camera.Options, busy map[string]bool) (cams []*camera.Camera, problems []error) {
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
		openGroup(calib, byName[name], devices, opts, busy, cams, problems)
	}
	return cams, problems
}

// openGroup opens the cameras of a calibration that share a device name (indexes),
// see OpenCameras.
func openGroup(calib Calibration, indexes []int, devices []camera.Device, opts func(CameraCalibration) camera.Options, busy map[string]bool, cams []*camera.Camera, problems []error) {
	fail := func(err error) {
		for _, i := range indexes {
			problems[i] = err
		}
	}
	var candidates []*camera.Camera
	var openErr error
	matchOpts := opts(calib.Cameras[indexes[0]]) // only to compare the views
	for _, d := range devices {
		if d.Name != calib.Cameras[indexes[0]].DeviceName || busy[d.ID] {
			continue
		}
		cam, err := camera.Open(d.ID, matchOpts)
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
		var err error
		i := indexes[r]
		if f < 0 {
			problems[i] = ErrNotConnected
			continue
		}
		cams[i], problems[i], used[f] = candidates[f], mismatch, true
		if want := opts(calib.Cameras[i]); want != matchOpts {
			// Another size or frame rate than the views were taken with.
			id := candidates[f].Device.ID
			candidates[f].Close()
			if cams[i], _, err = OpenCamera(id, want); err != nil {
				problems[i] = err
			}
		}
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
	deadline := time.Now().Add(maxSettleTime) // for all cameras: they adjust at the same time
	time.Sleep(settleTime)
	frames := make([]*image.RGBA, len(cams))
	for i, c := range cams {
		img, err := settled(c, deadline)
		if err != nil {
			return nil, err
		}
		frames[i] = img
		_ = c.LockExposure() // without the lock, detection works as before
	}
	return frames, nil
}

// settled reads frames until the camera's brightness stops changing or the deadline
// passed, and returns the last one.
func settled(c *camera.Camera, deadline time.Time) (*image.RGBA, error) {
	img, err := c.Read()
	for err == nil && time.Now().Before(deadline) {
		time.Sleep(settleStep)
		var next *image.RGBA
		if next, err = c.Read(); err != nil {
			break
		}
		still := math.Abs(brightness(next)-brightness(img)) < settledChange
		img = next
		if still {
			break
		}
	}
	return img, err
}

// brightness returns the mean brightness of an image (0–255), from a sample of its pixels.
func brightness(img *image.RGBA) float64 {
	const step = 4 * 31 // every 31st pixel
	var sum, n int
	for i := 0; i+2 < len(img.Pix); i += step {
		sum += int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
		n += 3
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
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
