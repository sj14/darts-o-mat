// Package camui is the web interface of darts-o-cam (see internal/cam/camapp): add the board's
// cameras, calibrate them, connect to darts-o-mat and see the latest detections.
package camui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/internal/cam/camapp"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/i18n"
	"github.com/sj14/darts-o-mat/internal/common/sse"
	"github.com/sj14/darts-o-mat/internal/common/static"
)

// Targets are the points the user clicks for the rough calibration, in this order:
// where a wire crosses the outer edge of the double ring. The first four are spread
// around the board; the others replace points that are hidden in a camera's view.
var Targets = [][2]uint8{{20, 1}, {6, 10}, {3, 19}, {11, 14}, {13, 6}, {17, 3}, {8, 11}, {5, 20}, {1, 18}, {10, 15}, {19, 7}, {14, 9}}

// MinPoints is the number of clicked points needed for the rough calibration.
const MinPoints = 4

// minFoundShare is the share of the board's features that must be found in the image,
// or the grid is shown as wrong. Measured on 1280×720: good calibrations 96–98 %; one
// with a wire mixed up when clicking (1/18 instead of 20/1) 22 %, while its mean
// deviation of 1.7 px looked fine. The share of features far off didn't tell them
// apart: 4–16 % for good ones (varies from frame to frame), 25–35 % for that one.
const minFoundShare = 0.5

type UI struct {
	app *camapp.App

	mu sync.Mutex
	// session is the calibration in progress, per camera name.
	session map[string]*session
	// checks are the latest grid checks, per camera name.
	checks map[string]check
	// traced is the detection last computed again with its steps, see tracePage.
	traced traced
}

// traced is a detection computed again with its steps: the images are drawn once, the
// page and its images are separate requests.
type traced struct {
	id     int
	traces []camapp.CameraTrace
	images []*image.RGBA // DrawTrace of each camera, nil where it failed
}

// check is a refinement run on a camera's current frame, to show the features found.
type check struct {
	frame  *image.RGBA
	result vision.RefineResult
	err    error
}

// session is a camera's calibration in progress: the frame the user clicks on,
// the points so far and the next target.
type session struct {
	frame  *image.RGBA
	points []vision.CalibrationPoint
	target int
	// manual is set once the user is to click the wire crossings, because they chose to
	// or because the automatic calibration from one click on the double of the 20 failed;
	// before that the first click is that one.
	manual bool
	err    error // of the last attempt to finish, see calibrationError
}

// New serves the web interface of the app.
func New(app *camapp.App) *UI {
	return &UI{app: app, session: map[string]*session{}, checks: map[string]check{}}
}

func (ui *UI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(i18n.Middleware) // the browser's language; darts-o-cam has no accounts
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(static.FS)))
	r.Get("/", ui.home)
	r.Get("/board/{board}", ui.overview)
	r.Post("/board/{board}/detect", ui.setDetect)
	r.Post("/board/{board}/reconnect", ui.reconnect)
	r.Get("/board/{board}/events/stream", ui.eventStream)
	r.Post("/boards/add", ui.addBoard)
	r.Post("/boards/{board}/remove", ui.removeBoard)
	r.Get("/board/{board}/settings", ui.settingsPage)
	r.Post("/board/{board}/settings", ui.saveSettings)
	r.Post("/board/{board}/settings/defaults", ui.defaultDetection)
	r.Get("/devices", ui.devicesPage)
	r.Get("/devices/snapshot.jpg", ui.snapshot)
	r.Post("/devices/refresh", ui.refreshSnapshots)
	r.Post("/devices/add", ui.addCamera)
	r.Get("/cam/{name}/frame.jpg", ui.frame)
	r.Get("/cam/{name}/remove", ui.removePage)
	r.Post("/cam/{name}/remove", ui.removeCamera)
	r.Get("/cam/{name}/calibrate", ui.calibratePage)
	r.Get("/cam/{name}/clicked.jpg", ui.clickedImage)
	r.Post("/cam/{name}/start", ui.start)
	r.Post("/cam/{name}/auto", ui.auto)
	r.Post("/cam/{name}/manual", ui.byHand)
	r.Post("/cam/{name}/point", ui.point)
	r.Post("/cam/{name}/skip", ui.skip)
	r.Post("/cam/{name}/finish", ui.finish)
	r.Post("/cam/{name}/refine", ui.refineAgain)
	r.Get("/cam/{name}/check", ui.checkPage)
	r.Get("/cam/{name}/features.jpg", ui.featuresImage)
	r.Get("/event/{id}", ui.eventPage)
	r.Get("/event/{id}/cam/{i}.jpg", ui.eventImage)
	r.Get("/event/{id}/cam/{i}/{kind}.jpg", ui.eventFrame)
	r.Get("/event/{id}/trace", ui.tracePage)
	r.Get("/event/{id}/trace/{i}.jpg", ui.traceImage)
	r.Get("/event/{id}/trace/{i}/zoom.jpg", ui.traceZoom)
	return r
}

// board returns the board named in the URL, writing 404 if there is none.
func (ui *UI) board(w http.ResponseWriter, r *http.Request) (camapp.Board, bool) {
	id, _ := strconv.Atoi(chi.URLParam(r, "board"))
	for _, b := range ui.app.Boards() {
		if b.ID == id {
			return b, true
		}
	}
	http.NotFound(w, r)
	return camapp.Board{}, false
}

// camera returns the camera named in the URL, writing 404 if there is none.
func (ui *UI) camera(w http.ResponseWriter, r *http.Request) (camapp.CameraState, bool) {
	c, ok := ui.app.Camera(chi.URLParam(r, "name"))
	if !ok {
		http.NotFound(w, r)
	}
	return c, ok
}

// latest returns the camera's current frame, writing an error if there is none.
func latest(w http.ResponseWriter, c camapp.CameraState) (*image.RGBA, bool) {
	if c.Live == nil {
		http.Error(w, "camera not connected", http.StatusServiceUnavailable)
		return nil, false
	}
	img, err := c.Live.Latest()
	if img == nil {
		http.Error(w, fmt.Sprintf("no frame yet: %v", err), http.StatusServiceUnavailable)
		return nil, false
	}
	return img, true
}

// home leads to the first board.
func (ui *UI) home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, boardURL(ui.app.Boards()[0].ID), http.StatusSeeOther)
}

func (ui *UI) overview(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	m := OverviewModel{boardNav: ui.nav(b.ID), Dir: ui.app.Dir(), Status: ui.app.Status(b.ID)}
	for _, c := range ui.app.Cameras(b.ID) {
		m.Cameras = append(m.Cameras, cameraModel(T(r.Context()), c))
	}
	m.Events = ui.events(T(r.Context()), b.ID)
	render(w, r, Overview(m))
}

func cameraModel(t Texts, c camapp.CameraState) CameraModel {
	m := CameraModel{Name: c.Name, BoardID: c.BoardID, DeviceName: c.DeviceName, DeviceModel: c.DeviceModel, Open: c.Live != nil, Calibrated: c.Calibrated}
	if toBoard, err := c.ToBoard(); err == nil && c.Calibrated {
		m.Scale = t.Number(vision.PixelScale(toBoard), 2)
	}
	if c.Live != nil {
		m.Device = c.Live.Device().ID
		if c.Frame.X > 0 {
			m.Size = fmt.Sprintf("%d×%d", c.Frame.X, c.Frame.Y)
		}
		if !c.Calibrated && c.Width != 0 {
			// Calibrated, but on frames of another size.
			m.OtherSize = fmt.Sprintf("%d×%d", c.Width, c.Height)
		}
	}
	switch {
	case c.Problem == nil:
	case errors.Is(c.Problem, vision.ErrNotConnected):
		m.Problem = t.ProblemNotConnected()
	case errors.Is(c.Problem, vision.ErrAmbiguousCameras):
		m.Problem = t.ProblemMoved()
	default:
		m.Problem = c.Problem.Error()
	}
	return m
}

func (ui *UI) setDetect(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	if err := ui.app.SetDetect(b.ID, r.FormValue("on") == "true"); err != nil {
		log.Printf("detection: %v", err)
	}
	http.Redirect(w, r, boardURL(b.ID), http.StatusSeeOther)
}

// reconnect opens the cameras of the board again.
func (ui *UI) reconnect(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	if err := ui.app.Reconnect(b.ID); err != nil {
		log.Printf("reconnect: %v", err)
	}
	http.Redirect(w, r, boardURL(b.ID), http.StatusSeeOther)
}

func (ui *UI) settingsPage(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	m := ui.settingsModel(b)
	m.Saved = r.URL.Query().Has("saved")
	render(w, r, SettingsPage(m))
}

// settingsModel is the settings page of a board with the given settings.
func (ui *UI) settingsModel(b camapp.Board) SettingsModel {
	return SettingsModel{Boards: ui.app.Boards(), Board: b}
}

// nav returns the boards for the page of the board with the given ID.
func (ui *UI) nav(id int) boardNav {
	return boardNav{Boards: ui.app.Boards(), Current: id}
}

// Sizes are the frame sizes offered in the settings.
var Sizes = [][2]int{{640, 480}, {1280, 720}, {1920, 1080}}

func (ui *UI) saveSettings(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	s := camapp.Board{ID: b.ID, Name: r.FormValue("name"), Server: r.FormValue("server"), Token: r.FormValue("token")}
	if _, err := fmt.Sscanf(r.FormValue("size"), "%dx%d", &s.Width, &s.Height); err != nil {
		s.Width, s.Height = 0, 0
	}
	s.FPS, _ = strconv.Atoi(r.FormValue("fps"))
	s.Detection = detectionForm(r)
	ui.applySettings(w, r, s)
}

// defaultDetection resets the detection's thresholds to their defaults.
func (ui *UI) defaultDetection(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	b.Detection = vision.DefaultWatchOptions
	ui.applySettings(w, r, b)
}

// detectionForm reads the detection's thresholds from the settings form; a field that
// isn't a number becomes 0, which the app rejects.
func detectionForm(r *http.Request) vision.WatchOptions {
	num := func(name string) float64 {
		f, _ := strconv.ParseFloat(strings.Replace(r.FormValue(name), ",", ".", 1), 64)
		return f
	}
	color := func(name string) uint8 {
		return uint8(min(max(num(name), 0), 255))
	}
	return vision.WatchOptions{
		Detect: vision.DetectOptions{
			DiffThreshold: color("diff_threshold"),
			WeakThreshold: color("weak_threshold"),
			Blur:          num("blur"),
			MinArea:       num("min_area"),
			MinSpeck:      num("min_speck"),
			MinElongation: num("min_elongation"),
			Gap:           num("gap"),
			AxisDistance:  num("axis_distance"),
			TipShift:      num("tip_shift"),
		},
		SampleStep:      num("sample_step"),
		MotionThreshold: color("motion_threshold"),
		MotionArea:      num("motion_area"),
		StableFrames:    int(num("stable_frames")),
		TakeoutArea:     num("takeout_area"),
	}
}

func (ui *UI) applySettings(w http.ResponseWriter, r *http.Request, s camapp.Board) {
	if err := ui.app.SetBoard(s); err != nil {
		m := ui.settingsModel(s)
		m.Error = err.Error()
		if errors.Is(err, camapp.ErrInvalidSettings) {
			m.Error = T(r.Context()).ErrInvalidSettings(err.Error())
		}
		render(w, r, SettingsPage(m))
		return
	}
	http.Redirect(w, r, boardURL(s.ID)+"/settings?saved", http.StatusSeeOther)
}

// addBoard adds a board and goes on to its settings, to name it and connect it.
func (ui *UI) addBoard(w http.ResponseWriter, r *http.Request) {
	id, err := ui.app.AddBoard()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, boardURL(id)+"/settings", http.StatusSeeOther)
}

func (ui *UI) removeBoard(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	m := ui.settingsModel(b)
	switch err := ui.app.RemoveBoard(b.ID); {
	case errors.Is(err, camapp.ErrBoardHasCameras):
		m.Error = T(r.Context()).ErrBoardHasCameras(b.Name)
		render(w, r, SettingsPage(m))
	case errors.Is(err, camapp.ErrLastBoard):
		m.Error = T(r.Context()).ErrLastBoard()
		render(w, r, SettingsPage(m))
	case err != nil:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// devicesModel is the devices page; selected is the board preselected for new cameras.
func (ui *UI) devicesModel(selected int) DevicesModel {
	devices, err := ui.app.Devices()
	m := DevicesModel{boardNav: ui.nav(selected), Devices: devices}
	if m.Board().ID != selected { // unknown or none given: the first
		m.Current = m.Boards[0].ID
	}
	if err != nil {
		m.Error = err.Error()
	}
	return m
}

func (ui *UI) devicesPage(w http.ResponseWriter, r *http.Request) {
	selected, _ := strconv.Atoi(r.URL.Query().Get("board"))
	render(w, r, Devices(ui.devicesModel(selected)))
}

// snapshot serves a frame of a connected device, to tell the cameras apart.
func (ui *UI) snapshot(w http.ResponseWriter, r *http.Request) {
	img, err := ui.app.Snapshot(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	writeJPEG(w, img)
}

func (ui *UI) refreshSnapshots(w http.ResponseWriter, r *http.Request) {
	ui.app.ClearSnapshots()
	http.Redirect(w, r, "/devices", http.StatusSeeOther)
}

// addCamera adds a device to a board and goes on to calibrate it.
func (ui *UI) addCamera(w http.ResponseWriter, r *http.Request) {
	board, _ := strconv.Atoi(r.FormValue("board"))
	name, err := ui.app.AddCamera(board, r.FormValue("id"))
	if err != nil {
		m := ui.devicesModel(board)
		m.Error = T(r.Context()).ErrAddCamera(err.Error())
		render(w, r, Devices(m))
		return
	}
	http.Redirect(w, r, "/cam/"+name+"/calibrate", http.StatusSeeOther)
}

func (ui *UI) removePage(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	render(w, r, RemovePage(cameraModel(T(r.Context()), c)))
}

func (ui *UI) removeCamera(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	name := c.Name
	if err := ui.app.RemoveCamera(name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ui.mu.Lock()
	delete(ui.session, name)
	delete(ui.checks, name)
	ui.mu.Unlock()
	http.Redirect(w, r, boardURL(c.BoardID), http.StatusSeeOther)
}

// frame serves the camera's newest frame, with the board drawn on it if calibrated.
func (ui *UI) frame(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, ok := latest(w, c)
	if !ok {
		return
	}
	var out image.Image = img
	if c.Calibrated {
		if m, err := c.Mapping(); err == nil {
			if drawn, err := vision.DrawBoard(img, m, color.RGBA{0, 255, 255, 255}); err == nil {
				out = drawn
			}
		}
	}
	writeJPEG(w, out)
}

func calibrateURL(name string) string {
	return "/cam/" + name + "/calibrate"
}

// start begins a new calibration of a camera on its current frame.
func (ui *UI) start(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, ok := latest(w, c)
	if !ok {
		return
	}
	ui.mu.Lock()
	ui.session[c.Name] = &session{frame: img}
	ui.mu.Unlock()
	http.Redirect(w, r, calibrateURL(c.Name), http.StatusSeeOther)
}

func (ui *UI) calibratePage(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	s := ui.session[c.Name]
	ui.mu.Unlock()
	if s == nil {
		ui.start(w, r)
		return
	}
	m := CalibrateModel{Name: c.Name, BoardID: c.BoardID, Auto: !s.manual, Points: len(s.points), MinPoints: MinPoints, Error: calibrationError(T(r.Context()), s.err), Bounds: s.frame.Bounds()}
	if s.target < len(Targets) {
		m.Target = &Targets[s.target]
	}
	render(w, r, Calibrate(m))
}

// clickedImage serves the frame being calibrated with the clicked points marked.
func (ui *UI) clickedImage(w http.ResponseWriter, r *http.Request) {
	ui.mu.Lock()
	s := ui.session[chi.URLParam(r, "name")]
	ui.mu.Unlock()
	if s == nil {
		http.NotFound(w, r)
		return
	}
	out := image.NewRGBA(s.frame.Bounds())
	copy(out.Pix, s.frame.Pix)
	for _, p := range s.points {
		vision.DrawCross(out, p.Image, 10, color.RGBA{255, 0, 255, 255})
	}
	writeJPEG(w, out)
}

// auto calibrates from the one click on the double of the 20 (vision.AutoCalibrate) and
// saves the result if the grid fits the image. If not, the user is told and gets the
// wire crossings to click as before.
func (ui *UI) auto(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	x, errX := strconv.ParseFloat(r.FormValue("click.x"), 64)
	y, errY := strconv.ParseFloat(r.FormValue("click.y"), 64)
	log.Printf("%s: click on the double of the 20 at %.0f, %.0f", name, x, y)
	ui.mu.Lock()
	s := ui.session[name]
	ui.mu.Unlock()
	if s == nil || s.manual || errX != nil || errY != nil {
		http.Redirect(w, r, calibrateURL(name), http.StatusSeeOther)
		return
	}

	res, err := vision.AutoCalibrate(s.frame, dartboard.Point{X: x, Y: y})
	if err == nil && res.FoundShare() < minFoundShare {
		err = fmt.Errorf("found only %.0f %% of the board's features", res.FoundShare()*100)
	}
	if err == nil {
		err = ui.store(name, res.Points, res.RefineResult, s.frame)
	}
	if err != nil {
		log.Printf("%s: automatic calibration: %v", name, err)
		ui.mu.Lock()
		s.manual, s.err = true, errAutoFailed
		ui.mu.Unlock()
		http.Redirect(w, r, calibrateURL(name), http.StatusSeeOther)
		return
	}
	ui.mu.Lock()
	delete(ui.session, name)
	ui.mu.Unlock()
	http.Redirect(w, r, "/cam/"+name+"/check", http.StatusSeeOther) // show the result
}

// byHand switches from the click on the double of the 20 to the wire crossings.
func (ui *UI) byHand(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ui.mu.Lock()
	if s := ui.session[name]; s != nil {
		s.manual, s.err = true, nil
	}
	ui.mu.Unlock()
	http.Redirect(w, r, calibrateURL(name), http.StatusSeeOther)
}

// point records a click on the image for the current target. The image input of the
// form sends the click position in pixels of the image, which is shown in full size.
func (ui *UI) point(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	x, errX := strconv.ParseFloat(r.FormValue("click.x"), 64)
	y, errY := strconv.ParseFloat(r.FormValue("click.y"), 64)
	log.Printf("%s: click at %.0f, %.0f", name, x, y)
	ui.mu.Lock()
	if s := ui.session[name]; s != nil && errX == nil && errY == nil && s.target < len(Targets) {
		s.points = append(s.points, vision.CalibrationPoint{
			Image:   dartboard.Point{X: x, Y: y},
			Between: Targets[s.target],
			Ring:    vision.RingDoubleOuter,
		})
		s.target++
		s.err = nil
	}
	ui.mu.Unlock()
	http.Redirect(w, r, calibrateURL(name)+"#frame", http.StatusSeeOther) // back to the image, not the top of the page
}

// skip moves on to the next target, e.g. when the point is hidden by a dart.
func (ui *UI) skip(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ui.mu.Lock()
	if s := ui.session[name]; s != nil && s.target < len(Targets) {
		s.target++
	}
	ui.mu.Unlock()
	http.Redirect(w, r, calibrateURL(name)+"#frame", http.StatusSeeOther) // back to the image, not the top of the page
}

// finish refines the clicked calibration on the frame and saves it.
func (ui *UI) finish(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	ui.mu.Lock()
	s := ui.session[name]
	ui.mu.Unlock()
	if s == nil {
		http.Redirect(w, r, calibrateURL(name), http.StatusSeeOther)
		return
	}
	if err := ui.refine(name, s.points, s.frame); err != nil {
		ui.mu.Lock()
		s.err = err
		ui.mu.Unlock()
		http.Redirect(w, r, calibrateURL(name), http.StatusSeeOther)
		return
	}
	ui.mu.Lock()
	delete(ui.session, name)
	ui.mu.Unlock()
	http.Redirect(w, r, "/cam/"+name+"/check", http.StatusSeeOther) // show the result
}

// refineAgain refines a calibrated camera again on its current frame, keeping its
// points, e.g. after the camera was bumped a little.
func (ui *UI) refineAgain(w http.ResponseWriter, r *http.Request) {
	c, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, ok := latest(w, c)
	if !ok {
		return
	}
	if err := ui.refine(c.Name, c.Points, img); err != nil {
		log.Printf("refine %s: %v", c.Name, err)
	}
	http.Redirect(w, r, boardURL(c.BoardID), http.StatusSeeOther)
}

// Why finishing a calibration failed; calibrationError says it in the user's language.
var (
	errTooFewPoints  = fmt.Errorf("at least %d points needed", MinPoints)
	errPointsDontFit = errors.New("the points don't fit together")
	errAutoFailed    = errors.New("the board wasn't found automatically")
)

// refineError is a failed fine calibration.
type refineError struct{ err error }

func (e refineError) Error() string { return "fine calibration failed: " + e.err.Error() }
func (e refineError) Unwrap() error { return e.err }

// calibrationError is the message of an error from finishing a calibration, empty
// for none.
func calibrationError(t Texts, err error) string {
	var refine refineError
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errTooFewPoints):
		return t.ErrTooFewPoints(MinPoints)
	case errors.Is(err, errPointsDontFit):
		return t.ErrPointsDontFit()
	case errors.Is(err, errAutoFailed):
		return t.ErrAutoFailed()
	case errors.As(err, &refine):
		return t.ErrRefine(refine.err)
	}
	return err.Error()
}

// refine refines the clicked calibration on an image of the empty board and stores it.
func (ui *UI) refine(name string, points []vision.CalibrationPoint, img *image.RGBA) error {
	if len(points) < MinPoints {
		return errTooFewPoints
	}
	rough, err := vision.CameraCalibration{Name: name, Points: points}.FromPoints()
	if err != nil {
		return errPointsDontFit
	}
	res, err := vision.RefineCalibration(img, rough)
	if err != nil {
		return refineError{err}
	}
	return ui.store(name, points, res, img)
}

// store saves a refined calibration with its rough points and remembers its result for the
// check of the grid.
func (ui *UI) store(name string, points []vision.CalibrationPoint, res vision.RefineResult, img *image.RGBA) error {
	log.Printf("%s: %d board features, deviation %.2f px before, %.2f px after", name, res.Matches, res.RMSBefore, res.RMSAfter)
	if err := ui.app.Calibrate(name, points, res.Mapping(), img); err != nil {
		return err
	}
	ui.mu.Lock()
	ui.checks[name] = check{frame: img, result: res}
	ui.mu.Unlock()
	return nil
}

// checkPage runs the refinement on the camera's current frame without saving it, and shows
// the ring edges and wires found: whether the board's grid is recognized correctly.
func (ui *UI) checkPage(w http.ResponseWriter, r *http.Request) {
	cam, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, ok := latest(w, cam)
	if !ok {
		return
	}
	start, err := cam.ToBoard()
	c := check{frame: img, err: err}
	if err == nil {
		c.result, c.err = vision.RefineCalibration(img, start)
	}
	ui.mu.Lock()
	ui.checks[cam.Name] = c
	ui.mu.Unlock()

	m := CheckModel{Name: cam.Name, BoardID: cam.BoardID}
	if c.err != nil {
		m.Error = c.err.Error()
	} else {
		m.Matches, m.RMS = c.result.Matches, c.result.RMSAfter
		for _, f := range c.result.Features {
			switch {
			case f.Outlier:
				m.Outliers++
			case f.Deviation <= 1:
				m.Good++
			case f.Deviation <= 2:
				m.Fair++
			default:
				m.Poor++
			}
		}
		m.Found, m.Searched = len(c.result.Features), c.result.Searched
		m.Wrong = float64(m.Found) < minFoundShare*float64(m.Searched)
	}
	render(w, r, Check(m))
}

// featuresImage draws the latest check: the board's grid as the refined calibration
// expects it (cyan) and the features found, colored by how well they fit.
func (ui *UI) featuresImage(w http.ResponseWriter, r *http.Request) {
	ui.mu.Lock()
	c, ok := ui.checks[chi.URLParam(r, "name")]
	ui.mu.Unlock()
	if !ok || c.err != nil {
		http.NotFound(w, r)
		return
	}
	out, err := vision.DrawBoard(c.frame, c.result.Mapping(), color.RGBA{0, 200, 255, 255})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, f := range c.result.Features {
		col := color.RGBA{0, 220, 0, 255}
		switch {
		case f.Outlier:
			col = color.RGBA{255, 0, 0, 255}
		case f.Deviation > 2:
			col = color.RGBA{255, 120, 0, 255}
		case f.Deviation > 1:
			col = color.RGBA{255, 230, 0, 255}
		}
		dot(out, f.Found, col)
	}
	writeJPEG(w, out)
}

// dot draws a small filled square.
func dot(img *image.RGBA, p dartboard.Point, c color.Color) {
	x, y := int(p.X+0.5), int(p.Y+0.5)
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			img.Set(x+dx, y+dy, c)
		}
	}
}

func (ui *UI) events(t Texts, board int) []EventModel {
	var events []EventModel
	for _, e := range ui.app.Events(board) {
		events = append(events, eventModel(t, e))
	}
	return events
}

// eventStream sends the list of a board's latest detections whenever there is a new one
// (Server-Sent Events, event "events"), so the overview shows them without reloading.
func (ui *UI) eventStream(w http.ResponseWriter, r *http.Request) {
	b, ok := ui.board(w, r)
	if !ok {
		return
	}
	changes, stop := ui.app.WatchEvents()
	defer stop()
	sse.Stream(w, r, "events", changes, func() (string, error) {
		var buf bytes.Buffer
		if err := eventList(ui.events(T(r.Context()), b.ID)).Render(r.Context(), &buf); err != nil {
			log.Printf("render events: %v", err)
			return "", err
		}
		return buf.String(), nil
	})
}

func eventModel(t Texts, e camapp.Event) EventModel {
	m := EventModel{ID: e.ID, BoardID: e.BoardID, Time: e.Time.Format("15:04:05"), Takeout: e.Kind == vision.EventTakeout, Game: e.Game}
	if e.ReportErr != nil {
		m.ReportErr = e.ReportErr.Error()
	}
	if !m.Takeout {
		m.Label = dartLabel(t, e.Dart)
		m.Board, m.Located = e.Board, e.Located
		for i, tip := range e.Event.Cameras {
			m.Cameras = append(m.Cameras, CameraPoint{
				Index: i, Name: e.Cameras[i], Board: tip.Point, Label: dartLabel(t, dartboard.Score(tip.Point)),
				Radial: tip.Radial, Tangential: tip.Tangential, Hidden: tip.Hidden,
			})
		}
		slices.SortFunc(m.Cameras, func(a, b CameraPoint) int { return a.Index - b.Index })
	}
	for i, name := range e.Cameras {
		m.AllCameras = append(m.AllCameras, CameraPoint{Index: i, Name: name})
	}
	return m
}

func (ui *UI) eventPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	e, ok := ui.app.Event(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, r, EventPage(eventModel(T(r.Context()), e)))
}

// eventImage shows a camera's frame after the event, around the tip it found.
func (ui *UI) eventImage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	i, _ := strconv.Atoi(chi.URLParam(r, "i"))
	path, ok := ui.app.EventFrame(id, i, "after")
	if !ok {
		http.NotFound(w, r)
		return
	}
	e, _ := ui.app.Event(id)
	frame, err := camapp.ReadJPEG(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tip, found := e.Tips[i]
	if !found {
		http.ServeFile(w, r, path)
		return
	}
	vision.DrawCross(frame, tip, 12, color.RGBA{0, 255, 0, 255})
	// Show the area around the tip, where it matters.
	const w2, h2 = 200, 150
	x0 := min(max(int(tip.X)-w2, 0), frame.Bounds().Dx()-2*w2)
	y0 := min(max(int(tip.Y)-h2, 0), frame.Bounds().Dy()-2*h2)
	writeJPEG(w, frame.SubImage(image.Rect(x0, y0, x0+2*w2, y0+2*h2)))
}

// eventFrame serves a camera's whole frame before or after the event as a download, to
// examine a misdetection; named like the frames darts-o-cam -save writes (e.g.
// 028-cam3-before.jpg), so darts-o-cam analyze reads them.
func (ui *UI) eventFrame(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	i, _ := strconv.Atoi(chi.URLParam(r, "i"))
	path, ok := ui.app.EventFrame(id, i, chi.URLParam(r, "kind"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		log.Printf("render: %v", err)
	}
}

func writeJPEG(w http.ResponseWriter, img image.Image) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// traceFor returns the detection of an event computed again with its steps, computing it
// only if it isn't the one computed last (it takes a while).
func (ui *UI) traceFor(id int) (traced, error) {
	ui.mu.Lock()
	t := ui.traced
	ui.mu.Unlock()
	if t.id == id && t.traces != nil {
		return t, nil
	}
	traces, err := ui.app.Traces(id)
	if err != nil {
		return traced{}, err
	}
	t = traced{id: id, traces: traces, images: make([]*image.RGBA, len(traces))}
	for i, c := range traces {
		if c.Err == nil {
			t.images[i] = vision.DrawTrace(c.After, c.Trace)
		}
	}
	ui.mu.Lock()
	ui.traced = t
	ui.mu.Unlock()
	return t, nil
}

// tracePage shows how each camera detected a dart, step by step, computed again from the
// saved frames on request (not while playing: it costs as much as the detection).
func (ui *UI) tracePage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	e, ok := ui.app.Event(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	t, err := ui.traceFor(id)
	if errors.Is(err, camapp.ErrNoTrace) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	m := TraceModel{Event: eventModel(T(r.Context()), e)}
	for i, c := range t.traces {
		m.Cameras = append(m.Cameras, traceCameraModel(i, c))
	}
	render(w, r, TracePage(m))
}

// traceImage serves a camera's frame with the steps of its detection drawn in.
func (ui *UI) traceImage(w http.ResponseWriter, r *http.Request) {
	if img, _, ok := ui.traceImageOf(w, r); ok {
		writeJPEG(w, img)
	}
}

// traceZoom serves the part of traceImage around the tip, to look at it pixel by pixel
// (the page enlarges it).
func (ui *UI) traceZoom(w http.ResponseWriter, r *http.Request) {
	img, c, ok := ui.traceImageOf(w, r)
	if !ok {
		return
	}
	center := zoomCenter(c.Trace, img.Bounds())
	x0 := min(max(center.X-zoomW/2, 0), img.Bounds().Dx()-zoomW)
	y0 := min(max(center.Y-zoomH/2, 0), img.Bounds().Dy()-zoomH)
	writePNG(w, img.SubImage(image.Rect(x0, y0, x0+zoomW, y0+zoomH)))
}

// zoomW and zoomH are the size of the zoomed part, in image pixels.
const zoomW, zoomH = 120, 90

// zoomCenter is where to zoom in: the tip, else the dart's or the largest change's middle.
func zoomCenter(t vision.Trace, b image.Rectangle) image.Point {
	if t.Err == nil {
		return image.Pt(int(t.Tip.Tip.X), int(t.Tip.Tip.Y))
	}
	pts := t.Area
	if len(pts) == 0 && len(t.Pieces) > 0 {
		pts = t.Pieces[0]
	}
	if len(pts) == 0 {
		return image.Pt(b.Dx()/2, b.Dy()/2)
	}
	var sx, sy int
	for _, p := range pts {
		sx, sy = sx+p.X, sy+p.Y
	}
	return image.Pt(sx/len(pts), sy/len(pts))
}

// traceImageOf returns the drawn trace of the camera in the URL, writing 404 without one.
func (ui *UI) traceImageOf(w http.ResponseWriter, r *http.Request) (*image.RGBA, camapp.CameraTrace, bool) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	i, _ := strconv.Atoi(chi.URLParam(r, "i"))
	t, err := ui.traceFor(id)
	if err != nil || i < 0 || i >= len(t.images) || t.images[i] == nil {
		http.NotFound(w, r)
		return nil, camapp.CameraTrace{}, false
	}
	return t.images[i], t.traces[i], true
}

// writePNG serves an image losslessly, so its pixels stay sharp when enlarged.
func writePNG(w http.ResponseWriter, img image.Image) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// traceCameraModel is what the trace page says about a camera's detection.
func traceCameraModel(i int, c camapp.CameraTrace) TraceCamera {
	t := c.Trace
	m := TraceCamera{Index: i, Name: c.Name, Retried: t.Retried, Pieces: len(t.Pieces), Elongation: t.Elongation, Found: c.Err == nil && t.Err == nil}
	if c.Err != nil {
		m.Err = c.Err.Error()
	} else if t.Err != nil {
		m.Err = t.Err.Error()
	}
	if t.Changed != nil {
		m.Changed = t.Changed.Count()
	}
	m.Area, m.Seed, m.TipEnd = len(t.Area), len(t.Seed), len(t.TipEnd)
	return m
}
