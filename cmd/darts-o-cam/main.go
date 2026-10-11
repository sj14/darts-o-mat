// darts-o-cam reads the cameras of a dartboard and detects darts.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"os/signal"
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
	_ = fs.Parse(args)

	calib, err := vision.LoadCalibration(*calibPath)
	if err != nil {
		return err
	}

	var toBoard []vision.Homography
	var latest []chan *image.RGBA
	for _, c := range calib.Cameras {
		h, err := c.ToBoard()
		if err != nil {
			return err
		}
		toBoard = append(toBoard, h)

		cam, err := vision.OpenCamera(c.Device, *width, *height, *fps)
		if err != nil {
			return fmt.Errorf("camera %s: %w", c.Name, err)
		}
		defer cam.Close()

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
	frames := make([]*image.RGBA, len(latest))
	for {
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
