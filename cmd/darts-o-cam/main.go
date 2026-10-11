// darts-o-cam reads the cameras of a dartboard and detects darts.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/domain/throw"
	"github.com/sj14/darts-o-mat/pkg/vision"
)

const usage = `usage: darts-o-cam <command> [flags]

commands:
  overlay   draw the calibrated board onto a camera image to check the calibration
  score     map image points (x,y in pixels) of a camera onto the board and score them
  detect    watch all calibrated cameras and print every dart thrown
  snapshot  save one frame of every calibrated camera, e.g. as reference images
  analyze   explain the detection of events saved by detect -save
`

func main() {
	log.SetFlags(0)
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
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
	cam, err := findCamera(calib, *camera)
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
	cam, err := findCamera(calib, *camera)
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
		fmt.Printf("%s\t%.1f,%.1f mm\tr=%.1f\t%s\n", arg, b.X, b.Y, b.Radius(), dartLabel(d))
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
	_ = fs.Parse(args)

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}

	cams, err := vision.OpenCameras(calib, *width, *height, *fps)
	if err != nil {
		return err
	}

	var toBoard []vision.Homography
	var latest []chan *image.RGBA
	for i, c := range calib.Cameras {
		h, err := c.ToBoard()
		if err != nil {
			return err
		}
		toBoard = append(toBoard, h)

		cam := cams[i]
		defer cam.Close()
		log.Printf("%s: device %s", c.Name, cam.Device)

		// Keep only the newest frame, so a slow analysis never works on old frames.
		ch := make(chan *image.RGBA, 1)
		latest = append(latest, ch)
		go func(name string) {
			for {
				img, err := cam.Read()
				if err != nil {
					log.Printf("camera %s: %v", name, err)
					return
				}
				select {
				case <-ch:
				default:
				}
				ch <- img
			}
		}(c.Name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("watching %d cameras, waiting for darts (stop with Ctrl+C)", len(latest))
	w := vision.NewWatcher(toBoard, vision.DefaultWatchOptions)
	if *verbose {
		w.OnMotion = func() { log.Printf("motion") }
	}
	events := 0
	for {
		frames := make([]*image.RGBA, len(latest))
		for i, ch := range latest {
			select {
			case frames[i] = <-ch:
			case <-ctx.Done():
				return nil
			}
		}
		e := w.Feed(frames)
		if e == nil {
			continue
		}
		events++
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
			log.Printf("dart: %s at %.1f,%.1f mm (%d cameras:%s)", dartLabel(e.Dart), e.Board.X, e.Board.Y, len(e.Cameras), cams)
		case vision.EventTakeout:
			log.Printf("takeout")
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
	// Only the first frames are needed, after the exposure settled.
	const fps = 10
	cams, err := vision.OpenCameras(calib, 1280, 720, fps)
	if err != nil {
		return err
	}
	for i, cam := range cams {
		defer cam.Close()
		var img *image.RGBA
		for range fps {
			if img, err = cam.Read(); err != nil {
				return err
			}
		}
		path := filepath.Join(*dir, calib.Cameras[i].Name+".jpg")
		if err := writeJPEG(path, img); err != nil {
			return err
		}
		log.Printf("%s (device %s): %s", calib.Cameras[i].Name, cam.Device, path)
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
				points = append(points, b)
				line += fmt.Sprintf(", tip %.0f,%.0f px -> %.0f,%.0f mm %s", d.Tip.X, d.Tip.Y, b.X, b.Y, dartLabel(dartboard.Score(b)))
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
			fmt.Printf("event %d: %s at %.0f,%.0f mm\n", n, dartLabel(dartboard.Score(p)), p.X, p.Y)
		}
	}
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

func dartLabel(d throw.Dart) string {
	switch {
	case d.Miss:
		return "miss"
	case d.Multiplier == 2:
		return fmt.Sprintf("D%d", d.Number)
	case d.Multiplier == 3:
		return fmt.Sprintf("T%d", d.Number)
	}
	return fmt.Sprintf("S%d", d.Number)
}

func findCamera(c vision.Calibration, name string) (vision.CameraCalibration, error) {
	for _, cam := range c.Cameras {
		if cam.Name == name {
			return cam, nil
		}
	}
	return vision.CameraCalibration{}, fmt.Errorf("camera %q not in calibration", name)
}
