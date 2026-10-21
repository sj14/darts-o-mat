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

	"github.com/sj14/darts-o-mat/internal/cam/camera"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/notify"
)

// App runs darts-o-cam: it keeps the cameras of all boards open, so they can be added,
// calibrated and removed at any time, and runs a detection per board on its calibrated
// cameras. Every change takes effect right away: the detection of the board it concerns
// starts over with the new cameras, calibration or settings. It is safe for concurrent use.
type App struct {
	dir  string
	opts Options

	mu   sync.Mutex // guards cfg, cams and runs
	cfg  Config
	cams map[string]*boardCamera // the open cameras, by name
	runs map[int]*run            // the running detections, by board

	snapMu sync.Mutex
	snaps  map[string]*snapshot // device ID → frame of a device that isn't a board camera

	evMu     sync.Mutex // guards events, nextID, reports and statuses
	events   []Event    // newest last, of all boards
	nextID   int
	reports  map[int]reportResult // of events AddEvent doesn't keep yet, by ID
	statuses map[int]Status       // by board
	// newEvent is told about every new detection, see WatchEvents.
	newEvent notify.Signal
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

// boardCamera is a camera at runtime.
type boardCamera struct {
	live          *vision.Live // nil if it couldn't be opened
	width, height int          // of its frames
	problem       error        // why it isn't open, or that its view doesn't match (vision.ErrAmbiguousCameras)
}

// New starts darts-o-cam with the configuration in dir: it opens the cameras and starts
// the detections. Without a configuration it starts without cameras.
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
	a := &App{dir: dir, opts: opts, cfg: cfg, cams: map[string]*boardCamera{}, runs: map[int]*run{}, snaps: map[string]*snapshot{}, statuses: map[int]Status{}, reports: map[int]reportResult{}}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.openCameras(a.allBoards()...)
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

// openCameras opens the configured cameras of the boards together, so that identical
// cameras of several boards are told apart as well as those of one. Devices open already
// (for other boards) are left alone. a.mu must be held.
func (a *App) openCameras(boards ...*BoardConfig) {
	var cameras []vision.CameraCalibration
	options := map[string]camera.Options{}
	for _, b := range boards {
		cameras = append(cameras, b.Cameras...)
		for _, c := range b.Cameras {
			options[c.Name] = b.options()
		}
	}
	busy := map[string]bool{}
	for _, c := range a.cams {
		if c.live != nil {
			busy[c.live.Device().ID] = true
		}
	}
	cams, problems := vision.OpenCameras(vision.NewCalibration(a.dir, cameras), func(c vision.CameraCalibration) camera.Options { return options[c.Name] }, busy)
	for i, cam := range cams {
		name := cameras[i].Name
		bc := &boardCamera{problem: problems[i]}
		a.cams[name] = bc
		if cam == nil {
			log.Printf("%s: %v", name, problems[i])
			continue
		}
		bc.start(cam)
		log.Printf("%s: device %s %s", name, cam.Device.ID, cam.Format)
		if problems[i] != nil {
			log.Printf("%s: %v", name, problems[i])
		}
	}
}

// allBoards returns all boards. a.mu must be held.
func (a *App) allBoards() []*BoardConfig {
	boards := make([]*BoardConfig, len(a.cfg.Boards))
	for i := range boards {
		boards[i] = &a.cfg.Boards[i]
	}
	return boards
}

// start reads the opened camera in the background.
func (c *boardCamera) start(cam *camera.Camera) {
	c.live = vision.StartLive(cam)
	if img, _, err := c.live.Next(0); err == nil {
		c.width, c.height = img.Bounds().Dx(), img.Bounds().Dy()
	}
}

// closeCameras closes the cameras of all boards. a.mu must be held, the detections stopped.
func (a *App) closeCameras() {
	for i := range a.cfg.Boards {
		a.closeBoardCameras(&a.cfg.Boards[i])
	}
}

// closeBoardCameras closes the cameras of a board. a.mu must be held, its detection stopped.
func (a *App) closeBoardCameras(b *BoardConfig) {
	for _, c := range b.Cameras {
		if bc := a.cams[c.Name]; bc != nil && bc.live != nil {
			bc.live.Close()
		}
		delete(a.cams, c.Name)
	}
}

// save writes the configuration. a.mu must be held.
func (a *App) save() error {
	return a.cfg.save(a.dir)
}

// CameraState is a camera as the web interface shows it.
type CameraState struct {
	vision.CameraCalibration
	// BoardID is the board the camera belongs to.
	BoardID int
	Live    *vision.Live // nil if it isn't open
	// Frame is the size of its frames, zero if it isn't open.
	Frame image.Point
	// Calibrated is whether it is calibrated for the size of its frames.
	Calibrated bool
	// Problem is why it isn't open, or that its view doesn't match its reference image.
	Problem error
}

// state returns the state of the camera c of the board. a.mu must be held.
func (a *App) state(board int, c vision.CameraCalibration) CameraState {
	bc := a.cams[c.Name]
	s := CameraState{CameraCalibration: c, BoardID: board, Live: bc.live, Frame: image.Pt(bc.width, bc.height), Problem: bc.problem, Calibrated: c.Calibrated(bc.width, bc.height)}
	if bc.live != nil {
		if _, err := bc.live.Latest(); err != nil {
			s.Problem = err // e.g. unplugged
		}
	}
	return s
}

// Cameras returns the cameras of a board.
func (a *App) Cameras(board int) []CameraState {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(board)
	if b == nil {
		return nil
	}
	states := make([]CameraState, len(b.Cameras))
	for i, c := range b.Cameras {
		states[i] = a.state(board, c)
	}
	return states
}

// Camera returns the camera with the given name, of whichever board.
func (a *App) Camera(name string) (CameraState, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, i := a.cfg.camera(name)
	if b == nil {
		return CameraState{}, false
	}
	return a.state(b.ID, b.Cameras[i]), true
}

// Calibrate stores a camera's new calibration, made on frame, an image of the empty
// board: the frame becomes its reference view and the empty board the detection compares with.
func (a *App) Calibrate(name string, points []vision.CalibrationPoint, refined vision.Mapping, frame *image.RGBA) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, i := a.cfg.camera(name)
	if b == nil {
		return fmt.Errorf("no camera %q", name)
	}
	cam := b.Cameras[i]
	cam.Points, cam.Refined, cam.Correction = points, &refined.ToBoard, refined.Correction
	cam.Width, cam.Height = frame.Bounds().Dx(), frame.Bounds().Dy()
	cam.Reference, cam.Empty = name+".jpg", name+"-empty.jpg"
	calib := a.calibration()
	for _, file := range []string{cam.Reference, cam.Empty} {
		if err := calib.SaveImage(file, frame); err != nil {
			return err
		}
	}
	b.Cameras[i] = cam
	// The view matches the reference again.
	if bc := a.cams[name]; errors.Is(bc.problem, vision.ErrAmbiguousCameras) {
		bc.problem = nil
	}
	if err := a.save(); err != nil {
		return err
	}
	a.restartBoard(b)
	return nil
}

// Device is a camera connected to this computer.
type Device struct {
	camera.Device
	// Camera is the camera using the device, empty if it isn't used.
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

// usedDevices maps the device IDs of the open cameras to the cameras' names.
func (a *App) usedDevices() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	used := map[string]string{}
	for name, c := range a.cams {
		if c.live != nil {
			used[c.live.Device().ID] = name
		}
	}
	return used
}

// ErrNoBoard means there is no board with the given ID.
var ErrNoBoard = errors.New("no such board")

// DeviceInUseError means a device is a board's camera already.
type DeviceInUseError struct {
	Camera string // its name
}

func (e DeviceInUseError) Error() string {
	return "device is already camera " + e.Camera
}

// AddCamera adds the device with the given ID to a board as a new camera, which then
// needs to be calibrated. It returns the camera's name.
func (a *App) AddCamera(board int, id string) (string, error) {
	if name := a.usedDevices()[id]; name != "" {
		return "", DeviceInUseError{Camera: name}
	}
	a.mu.Lock()
	var opts camera.Options
	b := a.cfg.board(board)
	if b != nil {
		opts = b.options()
	}
	a.mu.Unlock()
	if b == nil {
		return "", ErrNoBoard
	}
	a.forgetSnapshot(id) // the board camera shows its frames from now on
	// Opening takes a moment until the exposure settled: not while holding a.mu.
	cam, frame, err := vision.OpenCamera(id, opts)
	if err != nil {
		return "", err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	b = a.cfg.board(board)
	if b == nil { // removed meanwhile
		cam.Close()
		return "", ErrNoBoard
	}
	name := a.freeName()
	cc := vision.CameraCalibration{Name: name, DeviceName: cam.Device.Name, DeviceModel: cam.Device.Model, Reference: name + ".jpg"}
	// Its view tells it apart from cameras with the same name at the next start.
	if err := a.calibration().SaveImage(cc.Reference, frame); err != nil {
		cam.Close()
		return "", err
	}
	bc := &boardCamera{}
	bc.start(cam)
	b.Cameras = append(b.Cameras, cc)
	a.cams[name] = bc
	log.Printf("%s: added device %s (%s) %s to %s", name, cam.Device.ID, cam.Device.Name, cam.Format, b.Name)
	return name, a.save()
}

// freeName returns the first of cam1, cam2, … not used by a camera of any board. a.mu
// must be held.
func (a *App) freeName() string {
	for n := 1; ; n++ {
		name := "cam" + strconv.Itoa(n)
		if b, _ := a.cfg.camera(name); b == nil {
			return name
		}
	}
}

// RemoveCamera removes a camera from its board, with its calibration and images.
func (a *App) RemoveCamera(name string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, i := a.cfg.camera(name)
	if b == nil {
		return fmt.Errorf("no camera %q", name)
	}
	a.stopBoard(b.ID)
	if live := a.cams[name].live; live != nil {
		live.Close()
	}
	delete(a.cams, name)
	cam := b.Cameras[i]
	for _, file := range []string{cam.Reference, cam.Empty} {
		if file != "" {
			_ = os.Remove(filepath.Join(a.dir, file))
		}
	}
	b.Cameras = slices.Delete(b.Cameras, i, i+1)
	err := a.save()
	a.restartBoard(b)
	return err
}

// Boards returns the boards with their settings.
func (a *App) Boards() []Board {
	a.mu.Lock()
	defer a.mu.Unlock()
	boards := make([]Board, len(a.cfg.Boards))
	for i, b := range a.cfg.Boards {
		boards[i] = b.Board
	}
	return boards
}

// Why settings can't be used; they all wrap ErrInvalidSettings. The web interface says
// them in the user's language.
var (
	ErrInvalidSettings = errors.New("invalid settings")
	ErrNoBoardName     = fmt.Errorf("%w: a board needs a name", ErrInvalidSettings)
	ErrBoardNameTaken  = fmt.Errorf("%w: another board has this name", ErrInvalidSettings)
	ErrServerAddress   = fmt.Errorf("%w: the server must be an address like http://localhost:3000", ErrInvalidSettings)
	ErrFrameSettings   = fmt.Errorf("%w: size or frame rate out of range", ErrInvalidSettings)
	ErrColorDifference = fmt.Errorf("%w: color differences must be at least 1", ErrInvalidSettings)
	ErrWeakAboveStrong = fmt.Errorf("%w: the weak color difference must not be above the strong one", ErrInvalidSettings)
	ErrDetectionSizes  = fmt.Errorf("%w: sizes of the dart detection out of range", ErrInvalidSettings)
	ErrTipShift        = fmt.Errorf("%w: the tip may be moved by at most %g mm", ErrInvalidSettings, MaxTipShift)
	ErrMotionSettings  = fmt.Errorf("%w: motion settings out of range (the takeout area must be above the motion area)", ErrInvalidSettings)
)

// SetBoard changes the settings of the board with s.ID. Its cameras are opened again for
// a new size or frame rate (a camera calibrated on another size needs a new calibration
// then); the other boards are left alone.
func (a *App) SetBoard(s Board) error {
	s.Name, s.Token = strings.TrimSpace(s.Name), strings.TrimSpace(s.Token)
	s.Server = strings.TrimSuffix(strings.TrimSpace(s.Server), "/")
	if s.Name == "" {
		return ErrNoBoardName
	}
	if s.Server != "" {
		if u, err := url.Parse(s.Server); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return ErrServerAddress
		}
	}
	if s.Width <= 0 || s.Height <= 0 || s.FPS < 1 || s.FPS > 60 {
		return ErrFrameSettings
	}
	if err := validDetection(s.Detection); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(s.ID)
	if b == nil {
		return ErrNoBoard
	}
	if slices.ContainsFunc(a.cfg.Boards, func(o BoardConfig) bool { return o.ID != s.ID && o.Name == s.Name }) {
		return ErrBoardNameTaken
	}
	reopen := s.Width != b.Width || s.Height != b.Height || s.FPS != b.FPS
	b.Board = s
	if err := a.save(); err != nil {
		return err
	}
	a.stopBoard(b.ID)
	if reopen {
		a.closeBoardCameras(b)
		a.openCameras(b)
	}
	a.start(b)
	return nil
}

// AddBoard adds a board without cameras and returns its ID.
func (a *App) AddBoard() (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := 1
	for _, b := range a.cfg.Boards {
		id = max(id, b.ID+1)
	}
	name := "Board " + strconv.Itoa(id)
	for n := id + 1; slices.ContainsFunc(a.cfg.Boards, func(b BoardConfig) bool { return b.Name == name }); n++ {
		name = "Board " + strconv.Itoa(n) // one was renamed to this
	}
	a.cfg.Boards = append(a.cfg.Boards, newBoard(id, name))
	a.start(&a.cfg.Boards[len(a.cfg.Boards)-1])
	return id, a.save()
}

// Why a board can't be removed.
var (
	ErrLastBoard       = errors.New("the last board can't be removed")
	ErrBoardHasCameras = errors.New("the board still has cameras")
)

// RemoveBoard removes a board; its cameras have to be removed before.
func (a *App) RemoveBoard(id int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(id)
	switch {
	case b == nil:
		return ErrNoBoard
	case len(a.cfg.Boards) == 1:
		return ErrLastBoard
	case len(b.Cameras) > 0:
		return ErrBoardHasCameras
	}
	a.stopBoard(id)
	a.cfg.Boards = slices.DeleteFunc(a.cfg.Boards, func(b BoardConfig) bool { return b.ID == id })
	a.evMu.Lock()
	delete(a.statuses, id)
	a.evMu.Unlock()
	return a.save()
}

// Reconnect opens the cameras of a board again, e.g. after one was plugged in or out.
func (a *App) Reconnect(board int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(board)
	if b == nil {
		return ErrNoBoard
	}
	a.stopBoard(b.ID)
	a.closeBoardCameras(b)
	a.openCameras(b)
	a.start(b)
	return nil
}

// MaxTipShift limits DetectOptions.TipShift (mm): more than a few millimetres is no
// longer a correction of the tip, but moves darts into other fields.
const MaxTipShift = 5.0

// validDetection checks that the thresholds can work: sizes are positive, the weak
// threshold is below the strong one.
func validDetection(o vision.WatchOptions) error {
	d := o.Detect
	switch {
	case d.DiffThreshold == 0 || d.WeakThreshold == 0 || o.MotionThreshold == 0:
		return ErrColorDifference
	case d.WeakThreshold > d.DiffThreshold:
		return ErrWeakAboveStrong
	case d.Blur < 0 || d.Gap < 0 || d.AxisDistance <= 0 || d.MinArea <= 0 || d.MinSpeck < 0 || d.MinElongation < 1:
		return ErrDetectionSizes
	case math.Abs(d.TipShift) > MaxTipShift:
		return ErrTipShift
	case o.SampleStep <= 0 || o.MotionArea <= 0 || o.TakeoutArea <= o.MotionArea || o.StableFrames < 1:
		return ErrMotionSettings
	}
	return nil
}

// SetDetect starts or stops the detection of a board.
func (a *App) SetDetect(board int, on bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	b := a.cfg.board(board)
	if b == nil {
		return ErrNoBoard
	}
	b.Detect = on
	err := a.save()
	a.restartBoard(b)
	return err
}
