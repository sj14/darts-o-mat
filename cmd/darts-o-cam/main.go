// darts-o-cam reads the cameras of a dartboard and detects darts.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sj14/darts-o-mat/pkg/camera"
	"github.com/sj14/darts-o-mat/pkg/camui"
	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

const usage = `usage: darts-o-cam <command> [flags]

commands:
  ui        web interface to calibrate the cameras (the calibration stays on this computer)
  overlay   draw the calibrated board onto a camera image to check the calibration
  score     map image points (x,y in pixels) of a camera onto the board and score them
  detect    watch all calibrated cameras and print every dart thrown
  snapshot  save one frame of every calibrated camera, e.g. as reference images
  analyze   explain the detection of events saved by detect -save
  calibrate refine the calibration automatically from images of the empty board
`

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "ui":
		err = runUI(os.Args[2:])
	case "overlay":
		err = overlay(os.Args[2:])
	case "score":
		err = score(os.Args[2:])
	case "detect":
		err = detect(os.Args[2:])
	case "snapshot":
		err = snapshot(os.Args[2:])
	case "analyze":
		err = analyze(os.Args[2:])
	case "calibrate":
		err = calibrate(os.Args[2:])
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func overlay(args []string) error {
	fs := flag.NewFlagSet("overlay", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	camera := fs.String("camera", "", "name of the camera in the calibration file")
	in := fs.String("image", "", "camera image (JPEG or PNG)")
	out := fs.String("out", "overlay.png", "output PNG")
	_ = fs.Parse(args)

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}
	cam, err := calib.Camera(*camera)
	if err != nil {
		return err
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		return err
	}
	toImage, err := toBoard.Inverse()
	if err != nil {
		return err
	}

	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}

	result := vision.DrawBoard(img, toImage, color.RGBA{0, 255, 255, 255})
	for _, p := range cam.Points {
		vision.DrawCross(result, p.Image, 6, color.RGBA{255, 0, 255, 255})
	}

	o, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer o.Close()
	return png.Encode(o, result)
}

func score(args []string) error {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	camera := fs.String("camera", "", "name of the camera in the calibration file")
	_ = fs.Parse(args)

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}
	cam, err := calib.Camera(*camera)
	if err != nil {
		return err
	}
	toBoard, err := cam.ToBoard()
	if err != nil {
		return err
	}

	for _, arg := range fs.Args() {
		var p dartboard.Point
		if _, err := fmt.Sscanf(arg, "%f,%f", &p.X, &p.Y); err != nil {
			return fmt.Errorf("point %q: want x,y", arg)
		}
		b := toBoard.Apply(p)
		d := dartboard.Score(b)
		fmt.Printf("%s\t%.1f,%.1f mm\tr=%.1f\t%s\n", arg, b.X, b.Y, b.Radius(), d.String())
	}
	return nil
}

func detect(args []string) error {
	fs := flag.NewFlagSet("detect", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	width := fs.Int("width", 1280, "frame width")
	height := fs.Int("height", 720, "frame height")
	fps := fs.Int("fps", 10, "frames per second to analyze")
	verbose := fs.Bool("v", false, "log whenever the board starts moving")
	saveDir := fs.String("save", "", "directory to save the camera frames of every event to, for debugging")
	serverURL := fs.String("server", "", "darts-o-mat server to report darts to, e.g. http://localhost:3000 (default: only print them)")
	token := fs.String("token", "", "the board's camera token, created on the server's boards page")
	uiAddr := fs.String("ui", "localhost:8090", "address of the calibration interface, empty to turn it off")
	_ = fs.Parse(args)
	if (*serverURL == "") != (*token == "") {
		return fmt.Errorf("-server and -token are only used together")
	}
	var srv *server
	if *serverURL != "" {
		srv = newServer(*serverURL, *token)
	}

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}

	cams, err := vision.OpenCameras(calib, *width, *height, *fps)
	if err != nil {
		return err
	}

	var toBoard []vision.Homography
	var lives []*vision.Live
	for i, c := range calib.Cameras {
		h, err := c.ToBoard()
		if err != nil {
			return err
		}
		toBoard = append(toBoard, h)
		// The calibration interface shares the cameras.
		live := vision.StartLive(cams[i])
		defer live.Close()
		lives = append(lives, live)
		log.Printf("%s: device %s", c.Name, live.Device().ID)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		for _, l := range lives {
			l.Close() // ends the waits for frames below
		}
	}()

	w := vision.NewWatcher(toBoard, vision.DefaultWatchOptions)
	w.Empty, err = loadEmpty(calib, *width, *height)
	if err != nil {
		return err
	}
	if w.Empty == nil {
		log.Printf("no images of the empty board in the calibration: start with an empty board, or pulling out darts that were already in it counts as a throw")
	}
	if *verbose {
		w.OnMotion = func() { log.Printf("motion") }
	}

	var ui *camui.UI
	if *uiAddr != "" {
		ui = camui.New(*calibPath, calib, lives)
		ui.OnCalibrated = w.SetToBoard
		go func() {
			log.Printf("calibration interface on http://%s", *uiAddr)
			if err := http.ListenAndServe(*uiAddr, ui.Routes()); err != nil {
				log.Printf("calibration interface: %v", err)
			}
		}()
	}

	log.Printf("watching %d cameras, waiting for darts (stop with Ctrl+C)", len(lives))
	events := 0
	seq := make([]uint64, len(lives))
	for {
		// Analyze every frame once: wait for a new one of each camera.
		frames := make([]*image.RGBA, len(lives))
		for i, l := range lives {
			frames[i], seq[i], err = l.Next(seq[i])
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("camera %s: %w", calib.Cameras[i].Name, err)
			}
		}
		e := w.Feed(frames)
		if e == nil {
			continue
		}
		events++
		if ui != nil {
			ui.AddEvent(e)
		}
		if *saveDir != "" {
			if err := saveEvent(*saveDir, events, calib, e); err != nil {
				log.Printf("save event: %v", err)
			}
		}
		switch e.Kind {
		case vision.EventDart:
			cams := ""
			for i, p := range e.Cameras {
				cams += fmt.Sprintf(" %s=%.0f,%.0f", calib.Cameras[i].Name, p.X, p.Y)
			}
			log.Printf("dart: %s at %.1f,%.1f mm (%d cameras:%s)", e.Dart.String(), e.Board.X, e.Board.Y, len(e.Cameras), cams)
			if srv != nil {
				// The position goes along for statistics and to see how far off the cameras
				// were when the dart gets corrected.
				d := e.Dart
				d.X, d.Y = &e.Board.X, &e.Board.Y
				if id, err := srv.dart(d); err != nil {
					log.Printf("  not entered: %v", err)
				} else {
					log.Printf("  entered in game %d", id)
				}
			}
		case vision.EventTakeout:
			log.Printf("takeout")
			if srv != nil {
				if _, err := srv.takeout(); err != nil {
					log.Printf("  not reported: %v", err)
				}
			}
		}
	}
}

func snapshot(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	dir := fs.String("out", ".", "directory for the images, named after the cameras")
	_ = fs.Parse(args)

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}
	// OpenCameras returns once the exposure settled.
	cams, err := vision.OpenCameras(calib, 1280, 720, 10)
	if err != nil {
		return err
	}
	for i, cam := range cams {
		defer cam.Close()
		img, err := cam.Read()
		if err != nil {
			return err
		}
		path := filepath.Join(*dir, calib.Cameras[i].Name+".jpg")
		if err := writeJPEG(path, img); err != nil {
			return err
		}
		log.Printf("%s (device %s): %s", calib.Cameras[i].Name, cam.Device.ID, path)
	}
	return nil
}

// analyze runs the detection on saved events again and draws what each camera saw:
// the changed area in red and the detected tip as a cross.
func analyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	dir := fs.String("dir", ".", "directory with the frames saved by detect -save")
	out := fs.String("out", "", "directory for the annotated images (default: -dir)")
	_ = fs.Parse(args)
	if *out == "" {
		*out = *dir
	}

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}
	opts := vision.DefaultDetectOptions
	for n := 1; ; n++ {
		var points []dartboard.Point
		found := false
		for _, cam := range calib.Cameras {
			prefix := filepath.Join(*dir, fmt.Sprintf("%03d-%s", n, cam.Name))
			before, err1 := readImage(prefix + "-before.jpg")
			after, err2 := readImage(prefix + "-after.jpg")
			if err1 != nil || err2 != nil {
				continue
			}
			found = true

			area := vision.ChangedArea(before, after, opts)
			annotated := image.NewRGBA(after.Bounds())
			copy(annotated.Pix, after.Pix)
			for _, p := range area {
				annotated.SetRGBA(p.X, p.Y, color.RGBA{255, 0, 0, 255})
			}

			line := fmt.Sprintf("event %d %s: changed %d px", n, cam.Name, len(area))
			if d, err := vision.DetectDart(before, after, opts); err != nil {
				line += ", " + err.Error()
			} else {
				toBoard, err := cam.ToBoard()
				if err != nil {
					return err
				}
				b := toBoard.Apply(d.Tip)
				line += fmt.Sprintf(", tip %.0f,%.0f px -> %.0f,%.0f mm %s", d.Tip.X, d.Tip.Y, b.X, b.Y, dartboard.Score(b).String())
				if b.Radius() > vision.MaxBoardRadius {
					line += " (off the board, ignored)"
				} else {
					points = append(points, b)
				}
				vision.DrawCross(annotated, d.Tip, 8, color.RGBA{0, 255, 0, 255})
			}
			fmt.Println(line)
			if err := writeJPEG(filepath.Join(*out, fmt.Sprintf("%03d-%s-analyzed.jpg", n, cam.Name)), annotated); err != nil {
				return err
			}
		}
		if !found {
			return nil
		}
		if p, err := vision.FuseTips(points); err == nil {
			fmt.Printf("event %d: %s at %.0f,%.0f mm\n", n, dartboard.Score(p).String(), p.X, p.Y)
		}
	}
}

// calibrate refines each camera's calibration, starting from its points, with an image
// of the empty board: either from -images (<camera name>.jpg) or taken live.
func calibrate(args []string) error {
	fs := flag.NewFlagSet("calibrate", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file")
	out := fs.String("out", "", "file to write the refined calibration to (default: overwrite -calibration)")
	imagesDir := fs.String("images", "", "directory with an image of the empty board per camera (default: take them live)")
	overlayDir := fs.String("overlay", "", "directory to write overlays of the refined calibration to, for checking")
	_ = fs.Parse(args)
	if *out == "" {
		*out = *calibPath
	}

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}

	images := make([]*image.RGBA, len(calib.Cameras))
	if *imagesDir != "" {
		for i, cam := range calib.Cameras {
			if images[i], err = readImage(filepath.Join(*imagesDir, cam.Name+".jpg")); err != nil {
				return err
			}
		}
	} else {
		// OpenCameras returns once the exposure settled.
		cams, err := vision.OpenCameras(calib, 1280, 720, 10)
		if err != nil {
			return err
		}
		for i, cam := range cams {
			if images[i], err = cam.Read(); err != nil {
				return err
			}
			cam.Close()
		}
	}

	for i := range calib.Cameras {
		cam := &calib.Cameras[i]
		rough, err := cam.FromPoints()
		if err != nil {
			return err
		}
		res, err := vision.RefineCalibration(images[i], rough)
		if err != nil {
			return fmt.Errorf("camera %s: %w", cam.Name, err)
		}
		cam.Refined = &res.ToBoard
		log.Printf("%s: %d board features, deviation %.2f px before, %.2f px after", cam.Name, res.Matches, res.RMSBefore, res.RMSAfter)

		// Keep the image of the empty board: the detection compares with it.
		cam.Empty = cam.Name + "-empty.jpg"
		if err := writeJPEG(filepath.Join(filepath.Dir(*out), cam.Empty), images[i]); err != nil {
			return err
		}

		if *overlayDir != "" {
			toImage, err := res.ToBoard.Inverse()
			if err != nil {
				return err
			}
			path := filepath.Join(*overlayDir, cam.Name+"-refined.jpg")
			if err := writeJPEG(path, vision.DrawBoard(images[i], toImage, color.RGBA{0, 255, 255, 255})); err != nil {
				return err
			}
		}
	}
	return calib.Save(*out)
}

func readImage(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return vision.ToRGBA(img), nil
}

// runUI serves the calibration web interface. Without a calibration file yet, it opens
// all cameras named -device-name and creates one.
func runUI(args []string) error {
	fs := flag.NewFlagSet("ui", flag.ExitOnError)
	calibPath := fs.String("calibration", "calibration.json", "calibration file, created if it doesn't exist")
	addr := fs.String("addr", "localhost:8090", "address to serve the web interface on")
	deviceName := fs.String("device-name", "Autodarts DIY Cam", "name of the board's cameras, for a new calibration")
	_ = fs.Parse(args)

	const width, height, fps = 1280, 720, 10
	calib, err := vision.LoadCalibration(*calibPath)
	var cams []*camera.Camera
	switch {
	case errors.Is(err, os.ErrNotExist):
		log.Printf("no calibration at %s yet, opening all cameras named %q", *calibPath, *deviceName)
		calib = vision.NewCalibration(*calibPath, *deviceName)
		cams, err = vision.OpenCamerasNamed(*deviceName, width, height, fps)
	case err != nil:
		return err
	case len(calib.Cameras) == 0:
		// The cameras were chosen, but none is calibrated yet.
		cams, err = vision.OpenCamerasNamed(calib.DeviceName, width, height, fps)
	default:
		cams, err = vision.OpenCameras(calib, width, height, fps)
		if errors.Is(err, vision.ErrAmbiguousCameras) {
			// E.g. the board was moved: open them in device order, the user recalibrates anyway.
			log.Printf("cameras don't match their reference images (%v), opening them in device order: recalibrate them", err)
			cams, err = vision.OpenCamerasNamed(calib.DeviceName, width, height, fps)
		}
	}
	if err != nil {
		// Still serve the interface, so the cameras can be chosen there.
		log.Printf("no cameras opened (%v): choose them in the interface and restart", err)
		cams = nil
	}

	var live []*vision.Live
	for _, c := range cams {
		l := vision.StartLive(c)
		defer l.Close()
		live = append(live, l)
	}
	ui := camui.New(*calibPath, calib, live)
	log.Printf("calibration interface on http://%s", *addr)
	return http.ListenAndServe(*addr, ui.Routes())
}

func writeJPEG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// saveEvent writes the frames before and after an event as JPEGs, e.g. 003-cam1-after.jpg.
func saveEvent(dir string, n int, calib vision.Calibration, e *vision.Event) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for i, cam := range calib.Cameras {
		for name, img := range map[string]*image.RGBA{"before": e.Before[i], "after": e.After[i]} {
			if err := writeJPEG(filepath.Join(dir, fmt.Sprintf("%03d-%s-%s.jpg", n, cam.Name, name)), img); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadEmpty loads the images of the empty board of all cameras, nil if one is missing.
func loadEmpty(calib vision.Calibration, width, height int) ([]*image.RGBA, error) {
	var empty []*image.RGBA
	for _, cam := range calib.Cameras {
		img, err := calib.LoadEmpty(cam)
		if err != nil {
			return nil, err
		}
		if img == nil {
			return nil, nil
		}
		if img.Bounds().Dx() != width || img.Bounds().Dy() != height {
			return nil, fmt.Errorf("camera %s: image of the empty board is %dx%d, frames are %dx%d", cam.Name, img.Bounds().Dx(), img.Bounds().Dy(), width, height)
		}
		empty = append(empty, img)
	}
	return empty, nil
}
