package camapp

import (
	"errors"
	"fmt"
	"image"
	"log"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/sj14/darts-o-mat/pkg/camera"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

// App runs darts-o-cam: it keeps the board's cameras open, so they can be added,
// calibrated and removed at any time, and runs the detection on the calibrated ones.
// Every change takes effect right away: the detection starts over with the new
// cameras, calibration or settings. It is safe for concurrent use.
type App struct {
	dir  string
	opts Options

	mu   sync.Mutex // guards cfg, cams and run
	cfg  Config
	cams []*boardCamera // the open cameras, parallel to cfg.Cameras
	run  *run           // the running detection, nil if stopped

	snapMu sync.Mutex
	snaps  map[string]*snapshot // device ID → frame of a device that isn't a board camera

	evMu   sync.Mutex // guards events, nextID and status
	events []Event    // newest last
	nextID int
	status Status
}

// Options are settings of darts-o-cam that aren't kept in the configuration.
type Options struct {
	// FrameDir is where the frames of the detections are saved (before and after, per
	// camera), e.g. in the user's cache directory; empty: a folder "events" in the
	// configuration directory. Unless KeepFrames, only the frames of the detections shown
	// are kept, and the folder is emptied at the start (the numbers start over).
	FrameDir   string
	KeepFrames bool
	// Verbose logs whenever the board starts moving.
	Verbose bool
}

// boardCamera is a camera of the board at runtime.
type boardCamera struct {
	live          *vision.Live // nil if it couldn't be opened
	width, height int          // of its frames
	problem       error        // why it isn't open, or that its view doesn't match (vision.ErrAmbiguousCameras)
}

// New starts darts-o-cam with the configuration in dir: it opens the board's cameras
// and starts the detection. Without a configuration it starts without cameras.
func New(dir string, opts Options) (*App, error) {
	cfg, err := LoadConfig(dir)
	if err != nil {
		return nil, err
	}
	// The cameras' images are saved there before the configuration is.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if opts.FrameDir == "" {
		opts.FrameDir = filepath.Join(dir, "events")
	}
	if !opts.KeepFrames {
		if err := os.RemoveAll(opts.FrameDir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(opts.FrameDir, 0o755); err != nil {
		return nil, err
	}
	a := &App{dir: dir, opts: opts, cfg: cfg, snaps: map[string]*snapshot{}}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.openCameras()
	a.restart()
	return a, nil
}

// Dir is the directory with the configuration and the cameras' images.
func (a *App) Dir() string {
	return a.dir
}

// Close stops the detection and closes the cameras.
func (a *App) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stop()
	a.closeCameras()
}

// calibration returns the configured cameras as a vision.Calibration. a.mu must be held.
func (a *App) calibration() vision.Calibration {
	return a.cfg.Calibration(a.dir)
}

// openCameras opens the configured cameras. a.mu must be held.
func (a *App) openCameras() {
	opts := camera.Options{Width: a.cfg.Width, Height: a.cfg.Height, FPS: a.cfg.FPS}
	cams, problems := vision.OpenCameras(a.calibration(), opts)
	a.cams = make([]*boardCamera, len(cams))
	for i, cam := range cams {
		a.cams[i] = &boardCamera{problem: problems[i]}
		if cam == nil {
			log.Printf("%s: %v", a.cfg.Cameras[i].Name, problems[i])
			continue
		}
		a.cams[i].start(cam)
		log.Printf("%s: device %s %s", a.cfg.Cameras[i].Name, cam.Device.ID, cam.Format)
		if problems[i] != nil {
			log.Printf("%s: %v", a.cfg.Cameras[i].Name, problems[i])
		}
	}
}

// start reads the opened camera in the background.
func (c *boardCamera) start(cam *camera.Camera) {
	c.live = vision.StartLive(cam)
	if img, _, err := c.live.Next(0); err == nil {
		c.width, c.height = img.Bounds().Dx(), img.Bounds().Dy()
	}
}

// closeCameras closes the board's cameras. a.mu must be held, the detection stopped.
func (a *App) closeCameras() {
	for _, c := range a.cams {
		if c.live != nil {
			c.live.Close()
		}
	}
	a.cams = nil
}

// save writes the configuration. a.mu must be held.
func (a *App) save() error {
	return a.cfg.save(a.dir)
}

// CameraState is a camera of the board as the web interface shows it.
type CameraState struct {
	vision.CameraCalibration
	Live *vision.Live // nil if it isn't open
	// Frame is the size of its frames, zero if it isn't open.
	Frame image.Point
	// Calibrated is whether it is calibrated for the size of its frames.
	Calibrated bool
	// Problem is why it isn't open, or that its view doesn't match its reference image.
	Problem error
}

// Cameras returns the board's cameras.
func (a *App) Cameras() []CameraState {
	a.mu.Lock()
	defer a.mu.Unlock()
	states := make([]CameraState, len(a.cfg.Cameras))
	for i, c := range a.cfg.Cameras {
		bc := a.cams[i]
		states[i] = CameraState{CameraCalibration: c, Live: bc.live, Frame: image.Pt(bc.width, bc.height), Problem: bc.problem, Calibrated: c.Calibrated(bc.width, bc.height)}
		if bc.live != nil {
			if _, err := bc.live.Latest(); err != nil {
				states[i].Problem = err // e.g. unplugged
			}
		}
	}
	return states
}

// Camera returns the board camera with the given name.
func (a *App) Camera(name string) (CameraState, bool) {
	for _, c := range a.Cameras() {
		if c.Name == name {
			return c, true
		}
	}
	return CameraState{}, false
}

// index returns the index of the camera named name, -1 if there is none. a.mu must be held.
func (a *App) index(name string) int {
	return slices.IndexFunc(a.cfg.Cameras, func(c vision.CameraCalibration) bool { return c.Name == name })
}

// Calibrate stores a camera's new calibration, made on frame, an image of the empty
// board: the frame becomes its reference view and the empty board the detection compares with.
func (a *App) Calibrate(name string, points []vision.CalibrationPoint, refined vision.Mapping, frame *image.RGBA) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	i := a.index(name)
	if i < 0 {
		return fmt.Errorf("no camera %q", name)
	}
	cam := a.cfg.Cameras[i]
	cam.Points, cam.Refined, cam.Correction = points, &refined.ToBoard, refined.Correction
	cam.Width, cam.Height = frame.Bounds().Dx(), frame.Bounds().Dy()
	cam.Reference, cam.Empty = name+".jpg", name+"-empty.jpg"
	calib := a.calibration()
	for _, file := range []string{cam.Reference, cam.Empty} {
		if err := calib.SaveImage(file, frame); err != nil {
			return err
		}
	}
	a.cfg.Cameras[i] = cam
	// The view matches the reference again.
	if errors.Is(a.cams[i].problem, vision.ErrAmbiguousCameras) {
		a.cams[i].problem = nil
	}
	if err := a.save(); err != nil {
		return err
	}
	a.restart()
	return nil
}

// Device is a camera connected to this computer.
type Device struct {
	camera.Device
	// Camera is the board camera using the device, empty if it isn't one.
	Camera string
}

// Devices lists the cameras connected to this computer.
func (a *App) Devices() ([]Device, error) {
	devices, err := camera.Devices()
	if err != nil {
		return nil, err
	}
	used := a.usedDevices()
	var list []Device
	for _, d := range devices {
		list = append(list, Device{Device: d, Camera: used[d.ID]})
	}
	return list, nil
}

// usedDevices maps the device IDs of the open board cameras to the cameras' names.
func (a *App) usedDevices() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	used := map[string]string{}
	for i, c := range a.cams {
		if c.live != nil {
			used[c.live.Device().ID] = a.cfg.Cameras[i].Name
		}
	}
	return used
}

// AddCamera adds the device with the given ID to the board as a new camera, which then
// needs to be calibrated. It returns the camera's name.
func (a *App) AddCamera(id string) (string, error) {
	if name := a.usedDevices()[id]; name != "" {
		return "", fmt.Errorf("device is already camera %s", name)
	}
	a.forgetSnapshot(id) // the board camera shows its frames from now on
	a.mu.Lock()
	opts := camera.Options{Width: a.cfg.Width, Height: a.cfg.Height, FPS: a.cfg.FPS}
	a.mu.Unlock()
	// Opening takes a moment until the exposure settled: not while holding a.mu.
	cam, frame, err := vision.OpenCamera(id, opts)
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	name := a.freeName()
	cc := vision.CameraCalibration{Name: name, DeviceName: cam.Device.Name, DeviceModel: cam.Device.Model, Reference: name + ".jpg"}
	// Its view tells it apart from cameras with the same name at the next start.
	if err := a.calibration().SaveImage(cc.Reference, frame); err != nil {
		cam.Close()
		return "", err
	}
	bc := &boardCamera{}
	bc.start(cam)
	a.cfg.Cameras = append(a.cfg.Cameras, cc)
	a.cams = append(a.cams, bc)
	log.Printf("%s: added device %s (%s) %s", name, cam.Device.ID, cam.Device.Name, cam.Format)
	return name, a.save()
}

// freeName returns the first of cam1, cam2, … not used by a camera. a.mu must be held.
func (a *App) freeName() string {
	for n := 1; ; n++ {
		if name := "cam" + strconv.Itoa(n); a.index(name) < 0 {
			return name
		}
	}
}

// RemoveCamera removes a camera from the board, with its calibration and images.
func (a *App) RemoveCamera(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	i := a.index(name)
	if i < 0 {
		return fmt.Errorf("no camera %q", name)
	}
	a.stop()
	if live := a.cams[i].live; live != nil {
		live.Close()
	}
	cam := a.cfg.Cameras[i]
	for _, file := range []string{cam.Reference, cam.Empty} {
		if file != "" {
			_ = os.Remove(filepath.Join(a.dir, file))
		}
	}
	a.cfg.Cameras = slices.Delete(a.cfg.Cameras, i, i+1)
	a.cams = slices.Delete(a.cams, i, i+1)
	err := a.save()
	a.restart()
	return err
}

// Settings are what can be set in the web interface.
type Settings struct {
	Server, Token string
	// Width and Height are the wanted frame size; each camera uses its own mode closest
	// to it (see camera.Options).
	Width, Height, FPS int
	Detection          vision.WatchOptions
}

// Settings returns the current settings.
func (a *App) Settings() Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Settings{Server: a.cfg.Server, Token: a.cfg.Token, Width: a.cfg.Width, Height: a.cfg.Height, FPS: a.cfg.FPS, Detection: a.cfg.Detection}
}

// ErrInvalidSettings is returned for settings that can't be used.
var ErrInvalidSettings = errors.New("invalid settings")

// SetSettings changes the settings. Cameras are opened again for a new size or frame
// rate (a camera calibrated on another size needs a new calibration then).
func (a *App) SetSettings(s Settings) error {
	s.Server, s.Token = strings.TrimSuffix(strings.TrimSpace(s.Server), "/"), strings.TrimSpace(s.Token)
	if s.Server != "" {
		if u, err := url.Parse(s.Server); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: the server must be an address like http://localhost:3000", ErrInvalidSettings)
		}
	}
	if s.Width <= 0 || s.Height <= 0 || s.FPS < 1 || s.FPS > 60 {
		return fmt.Errorf("%w: size or frame rate out of range", ErrInvalidSettings)
	}
	if err := validDetection(s.Detection); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidSettings, err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	reopen := s.Width != a.cfg.Width || s.Height != a.cfg.Height || s.FPS != a.cfg.FPS
	a.cfg.Server, a.cfg.Token = s.Server, s.Token
	a.cfg.Width, a.cfg.Height, a.cfg.FPS = s.Width, s.Height, s.FPS
	a.cfg.Detection = s.Detection
	if err := a.save(); err != nil {
		return err
	}
	a.stop()
	if reopen {
		a.closeCameras()
		a.openCameras()
	}
	a.restart()
	return nil
}

// Reconnect opens the cameras again, e.g. after one was plugged in or out.
func (a *App) Reconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stop()
	a.closeCameras()
	a.openCameras()
	a.restart()
}

// maxTipShift limits DetectOptions.TipShift (mm): more than a few millimetres is no
// longer a correction of the tip, but moves darts into other fields.
const maxTipShift = 5.0

// validDetection checks that the thresholds can work: sizes are positive, the weak
// threshold is below the strong one.
func validDetection(o vision.WatchOptions) error {
	d := o.Detect
	switch {
	case d.DiffThreshold == 0 || d.WeakThreshold == 0 || o.MotionThreshold == 0:
		return errors.New("color differences must be at least 1")
	case d.WeakThreshold > d.DiffThreshold:
		return errors.New("the weak color difference must not be above the strong one")
	case d.Blur < 0 || d.Gap < 0 || d.AxisDistance <= 0 || d.MinArea <= 0 || d.MinSpeck < 0 || d.MinElongation < 1:
		return errors.New("sizes of the dart detection out of range")
	case math.Abs(d.TipShift) > maxTipShift:
		return fmt.Errorf("the tip may be moved by at most %g mm", maxTipShift)
	case o.SampleStep <= 0 || o.MotionArea <= 0 || o.TakeoutArea <= o.MotionArea || o.StableFrames < 1:
		return errors.New("motion settings out of range (the takeout area must be above the motion area)")
	}
	return nil
}

// SetDetect starts or stops the detection.
func (a *App) SetDetect(on bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Detect = on
	err := a.save()
	a.restart()
	return err
}
