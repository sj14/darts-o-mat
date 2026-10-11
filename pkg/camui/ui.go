// Package camui is the local web interface of darts-o-cam to calibrate a board's cameras.
// The calibration stays on the computer with the cameras, so the same board works with
// any darts-o-mat server.
package camui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/frontend/static"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

// Targets are the points the user clicks for the rough calibration, in this order:
// where a wire crosses the outer edge of the double ring. The first four are spread
// around the board; the others replace points that are hidden in a camera's view.
var Targets = [][2]uint8{{20, 1}, {6, 10}, {3, 19}, {11, 14}, {13, 6}, {17, 3}, {8, 11}, {5, 20}, {1, 18}, {10, 15}, {19, 7}, {14, 9}}

// MinPoints is the number of clicked points needed for the rough calibration.
const MinPoints = 4

type UI struct {
	// OnCalibrated is called after a camera was calibrated, e.g. to use the new
	// calibration for the running detection right away.
	OnCalibrated func(camera int, toBoard vision.Homography)

	path  string
	cams  []*vision.Live
	mu    sync.Mutex
	calib vision.Calibration
	// session is the calibration in progress, per camera.
	session map[int]*session
	// events are the latest detections, newest last.
	events []shownEvent
	nextID int
	// checks are the latest grid checks, per camera.
	checks map[int]check
}

// shownEvent is a detection kept to show it.
type shownEvent struct {
	ID    int
	Time  time.Time
	Event *vision.Event
}

// check is a refinement run on a camera's current frame, to show the features found.
type check struct {
	frame  *image.RGBA
	result vision.RefineResult
	err    error
}

// maxEvents is how many detections are kept to show them.
const maxEvents = 20

// AddEvent shows a detection.
func (ui *UI) AddEvent(e *vision.Event) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.nextID++
	ui.events = append(ui.events, shownEvent{ID: ui.nextID, Time: time.Now(), Event: e})
	if len(ui.events) > maxEvents {
		ui.events = ui.events[len(ui.events)-maxEvents:]
	}
}

func (ui *UI) event(id int) (shownEvent, bool) {
	for _, e := range ui.events {
		if e.ID == id {
			return e, true
		}
	}
	return shownEvent{}, false
}

// session is a camera's calibration in progress: the frame the user clicks on,
// the points so far and the next target.
type session struct {
	frame  *image.RGBA
	points []vision.CalibrationPoint
	target int
	err    string
}

// New serves the calibration at path for the cameras, in the calibration's order.
// If the calibration has fewer cameras, the missing ones are added as cam1, cam2, ….
func New(path string, calib vision.Calibration, cams []*vision.Live) *UI {
	for i := len(calib.Cameras); i < len(cams); i++ {
		calib.Cameras = append(calib.Cameras, vision.CameraCalibration{Name: fmt.Sprintf("cam%d", i+1)})
	}
	return &UI{path: path, cams: cams, calib: calib, session: map[int]*session{}, checks: map[int]check{}}
}

func (ui *UI) Routes() http.Handler {
	r := chi.NewRouter()
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServerFS(static.FS)))
	r.Get("/", ui.overview)
	r.Get("/cam/{i}/frame.jpg", ui.frame)
	r.Get("/cam/{i}/calibrate", ui.calibratePage)
	r.Get("/cam/{i}/clicked.jpg", ui.clickedImage)
	r.Post("/cam/{i}/start", ui.start)
	r.Post("/cam/{i}/point", ui.point)
	r.Post("/cam/{i}/skip", ui.skip)
	r.Post("/cam/{i}/finish", ui.finish)
	r.Post("/cam/{i}/refine", ui.refineAgain)
	r.Get("/cam/{i}/check", ui.checkPage)
	r.Get("/cam/{i}/features.jpg", ui.featuresImage)
	r.Get("/event/{id}", ui.eventPage)
	r.Get("/event/{id}/cam/{i}.jpg", ui.eventImage)
	return r
}

// camera returns the index from the URL, writing 404 if it doesn't exist.
func (ui *UI) camera(w http.ResponseWriter, r *http.Request) (int, bool) {
	i, err := strconv.Atoi(chi.URLParam(r, "i"))
	if err != nil || i < 0 || i >= len(ui.cams) {
		http.NotFound(w, r)
		return 0, false
	}
	return i, true
}

func (ui *UI) overview(w http.ResponseWriter, r *http.Request) {
	ui.mu.Lock()
	var cams []CameraModel
	for i, c := range ui.calib.Cameras {
		if i >= len(ui.cams) {
			break // a camera of the calibration isn't connected
		}
		cams = append(cams, CameraModel{Index: i, Name: c.Name, Device: ui.cams[i].Device(), Calibrated: c.Refined != nil})
	}
	var events []EventModel
	for i := len(ui.events) - 1; i >= 0; i-- {
		events = append(events, ui.eventModel(ui.events[i]))
	}
	ui.mu.Unlock()
	render(w, r, Overview(OverviewModel{Path: ui.path, Cameras: cams, Events: events}))
}

// frame serves the camera's newest frame, with the board drawn on it if calibrated.
func (ui *UI) frame(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, err := ui.cams[i].Latest()
	if img == nil {
		http.Error(w, fmt.Sprintf("no frame yet: %v", err), http.StatusServiceUnavailable)
		return
	}
	ui.mu.Lock()
	cam := ui.calib.Cameras[i]
	ui.mu.Unlock()
	var out image.Image = img
	if cam.Refined != nil {
		if toImage, err := cam.Refined.Inverse(); err == nil {
			out = vision.DrawBoard(img, toImage, color.RGBA{0, 255, 255, 255})
		}
	}
	writeJPEG(w, out)
}

// start begins a new calibration of a camera on its current frame.
func (ui *UI) start(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, _ := ui.cams[i].Latest()
	if img == nil {
		http.Error(w, "no frame yet", http.StatusServiceUnavailable)
		return
	}
	ui.mu.Lock()
	ui.session[i] = &session{frame: img}
	ui.mu.Unlock()
	http.Redirect(w, r, fmt.Sprintf("/cam/%d/calibrate", i), http.StatusSeeOther)
}

func (ui *UI) calibratePage(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	s := ui.session[i]
	cam := ui.calib.Cameras[i]
	ui.mu.Unlock()
	if s == nil {
		ui.start(w, r)
		return
	}
	m := CalibrateModel{Index: i, Name: cam.Name, Points: len(s.points), MinPoints: MinPoints, Error: s.err}
	if s.target < len(Targets) {
		m.Target = &Targets[s.target]
	}
	m.Bounds = s.frame.Bounds()
	render(w, r, Calibrate(m))
}

// clickedImage serves the frame being calibrated with the clicked points marked.
func (ui *UI) clickedImage(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	s := ui.session[i]
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

// point records a click on the image for the current target. The image input of the
// form sends the click position in pixels of the image, which is shown in full size.
func (ui *UI) point(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	x, errX := strconv.ParseFloat(r.FormValue("click.x"), 64)
	y, errY := strconv.ParseFloat(r.FormValue("click.y"), 64)
	ui.mu.Lock()
	if s := ui.session[i]; s != nil && errX == nil && errY == nil && s.target < len(Targets) {
		s.points = append(s.points, vision.CalibrationPoint{
			Image:   dartboard.Point{X: x, Y: y},
			Between: Targets[s.target],
			Ring:    vision.RingDoubleOuter,
		})
		s.target++
		s.err = ""
	}
	ui.mu.Unlock()
	http.Redirect(w, r, fmt.Sprintf("/cam/%d/calibrate", i), http.StatusSeeOther)
}

// skip moves on to the next target, e.g. when the point is hidden by a dart.
func (ui *UI) skip(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	if s := ui.session[i]; s != nil && s.target < len(Targets) {
		s.target++
	}
	ui.mu.Unlock()
	http.Redirect(w, r, fmt.Sprintf("/cam/%d/calibrate", i), http.StatusSeeOther)
}

// finish refines the clicked calibration on the frame and saves it.
func (ui *UI) finish(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	defer ui.mu.Unlock()
	s := ui.session[i]
	if s == nil {
		http.Redirect(w, r, fmt.Sprintf("/cam/%d/calibrate", i), http.StatusSeeOther)
		return
	}

	cam := ui.calib.Cameras[i]
	cam.Points = s.points
	cam.Refined = nil
	if err := ui.refine(i, &cam, s.frame); err != nil {
		s.err = err.Error()
		http.Redirect(w, r, fmt.Sprintf("/cam/%d/calibrate", i), http.StatusSeeOther)
		return
	}
	delete(ui.session, i)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// refineAgain refines a calibrated camera again on its current frame, keeping its
// points, e.g. after the camera was bumped a little.
func (ui *UI) refineAgain(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, _ := ui.cams[i].Latest()
	ui.mu.Lock()
	defer ui.mu.Unlock()
	cam := ui.calib.Cameras[i]
	if img == nil || len(cam.Points) < MinPoints {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if err := ui.refine(i, &cam, img); err != nil {
		log.Printf("refine %s: %v", cam.Name, err)
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// refine refines the camera's calibration on an image of the empty board and saves the
// calibration with the image as reference and empty board. ui.mu must be held.
func (ui *UI) refine(i int, cam *vision.CameraCalibration, img *image.RGBA) error {
	if len(cam.Points) < MinPoints {
		return fmt.Errorf("mindestens %d Punkte nötig", MinPoints)
	}
	rough, err := cam.FromPoints()
	if err != nil {
		return errors.New("die Punkte passen nicht zusammen, bitte neu beginnen")
	}
	res, err := vision.RefineCalibration(img, rough)
	if err != nil {
		return fmt.Errorf("Feinkalibrierung fehlgeschlagen (%v); ist das Board leer und gut zu sehen?", err)
	}
	cam.Refined = &res.ToBoard
	cam.Reference = cam.Name + ".jpg"
	cam.Empty = cam.Name + "-empty.jpg"
	for _, name := range []string{cam.Reference, cam.Empty} {
		if err := ui.calib.SaveImage(name, img); err != nil {
			return err
		}
	}
	ui.calib.Cameras[i] = *cam
	ui.checks[i] = check{frame: img, result: res}
	log.Printf("%s: %d board features, deviation %.2f px before, %.2f px after", cam.Name, res.Matches, res.RMSBefore, res.RMSAfter)
	if ui.OnCalibrated != nil {
		ui.OnCalibrated(i, res.ToBoard)
	}
	return ui.calib.Save(ui.path)
}

// checkPage runs the refinement on the camera's current frame without saving it, and shows
// the ring edges and wires found: whether the board's grid is recognized correctly.
func (ui *UI) checkPage(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	img, _ := ui.cams[i].Latest()
	ui.mu.Lock()
	cam := ui.calib.Cameras[i]
	ui.mu.Unlock()
	if img == nil {
		http.Error(w, "no frame yet", http.StatusServiceUnavailable)
		return
	}
	start, err := cam.ToBoard()
	c := check{frame: img, err: err}
	if err == nil {
		c.result, c.err = vision.RefineCalibration(img, start)
	}
	ui.mu.Lock()
	ui.checks[i] = c
	ui.mu.Unlock()

	m := CheckModel{Index: i, Name: cam.Name}
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
	}
	render(w, r, Check(m))
}

// featuresImage draws the latest check: the board's grid as the refined calibration
// expects it (cyan) and the features found, colored by how well they fit.
func (ui *UI) featuresImage(w http.ResponseWriter, r *http.Request) {
	i, ok := ui.camera(w, r)
	if !ok {
		return
	}
	ui.mu.Lock()
	c, ok := ui.checks[i]
	ui.mu.Unlock()
	if !ok || c.err != nil {
		http.NotFound(w, r)
		return
	}
	toImage, err := c.result.ToBoard.Inverse()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	out := vision.DrawBoard(c.frame, toImage, color.RGBA{0, 200, 255, 255})
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

func (ui *UI) eventModel(e shownEvent) EventModel {
	m := EventModel{ID: e.ID, Time: e.Time.Format("15:04:05"), Takeout: e.Event.Kind == vision.EventTakeout}
	if !m.Takeout {
		m.Label = DartLabel(e.Event.Dart)
		m.Board = e.Event.Board
		for i, p := range e.Event.Cameras {
			m.Cameras = append(m.Cameras, CameraPoint{Index: i, Name: ui.calib.Cameras[i].Name, Board: p, Label: DartLabel(dartboard.Score(p))})
		}
		slices.SortFunc(m.Cameras, func(a, b CameraPoint) int { return a.Index - b.Index })
	}
	return m
}

func (ui *UI) eventPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	ui.mu.Lock()
	e, ok := ui.event(id)
	var m EventModel
	if ok {
		m = ui.eventModel(e)
		for i := range ui.calib.Cameras {
			m.AllCameras = append(m.AllCameras, CameraPoint{Index: i, Name: ui.calib.Cameras[i].Name})
		}
	}
	ui.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	render(w, r, EventPage(m))
}

// eventImage shows a camera's frame after the event, around the tip it found.
func (ui *UI) eventImage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	i, err := strconv.Atoi(chi.URLParam(r, "i"))
	ui.mu.Lock()
	e, ok := ui.event(id)
	ui.mu.Unlock()
	if !ok || err != nil || i < 0 || i >= len(e.Event.After) {
		http.NotFound(w, r)
		return
	}
	frame := e.Event.After[i]
	out := image.NewRGBA(frame.Bounds())
	copy(out.Pix, frame.Pix)
	tip, found := e.Event.Tips[i]
	if found {
		vision.DrawCross(out, tip, 12, color.RGBA{0, 255, 0, 255})
		// Show the area around the tip, where it matters.
		const w2, h2 = 200, 150
		x0 := min(max(int(tip.X)-w2, 0), out.Bounds().Dx()-2*w2)
		y0 := min(max(int(tip.Y)-h2, 0), out.Bounds().Dy()-2*h2)
		writeJPEG(w, out.SubImage(image.Rect(x0, y0, x0+2*w2, y0+2*h2)))
		return
	}
	writeJPEG(w, out)
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

// DartLabel names a dart like the scoreboard does, e.g. T20, D16, Bull.
func DartLabel(d throw.Dart) string {
	switch {
	case d.Miss:
		return "daneben"
	case d.Number == throw.Bull && d.Multiplier == 2:
		return "Bullseye"
	case d.Number == throw.Bull:
		return "Bull"
	case d.Multiplier == 2:
		return fmt.Sprintf("D%d", d.Number)
	case d.Multiplier == 3:
		return fmt.Sprintf("T%d", d.Number)
	}
	return fmt.Sprintf("S%d", d.Number)
}
