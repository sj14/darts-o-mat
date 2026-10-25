// darts-o-cam watches the cameras of a dartboard, detects darts and reports them to
// darts-o-mat. Everything is set up in its web interface: start it and open the page.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/sj14/darts-o-mat/internal/cam/camapp"
	"github.com/sj14/darts-o-mat/internal/cam/camui"
	"github.com/sj14/darts-o-mat/internal/cam/vision"
	"github.com/sj14/darts-o-mat/internal/common/dartboard"
	"github.com/sj14/darts-o-mat/internal/common/profiling"
	"github.com/sj14/darts-o-mat/internal/common/update"
)

func main() {
	log.SetFlags(0)
	var err error
	if len(os.Args) > 1 && os.Args[1] == "analyze" {
		err = analyze(os.Args[2:])
	} else {
		err = run(os.Args[1:])
	}
	if err != nil {
		log.Fatal(err)
	}
}

// run starts darts-o-cam with its web interface.
func run(args []string) (err error) {
	fs := flag.NewFlagSet("darts-o-cam", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(fs.Output(), `usage: darts-o-cam [flags]
       darts-o-cam analyze [flags]   explain the detection of events saved with -save

Starts darts-o-cam and its web interface, where the cameras are added and calibrated.

`)
		fs.PrintDefaults()
	}
	defaultDir, err := camapp.DefaultDir()
	if err != nil {
		return err
	}
	dir := fs.String("dir", defaultDir, "directory for the configuration and the cameras' images")
	addr := fs.String("addr", "localhost:8090", "address of the web interface; e.g. :8090 to reach it from other devices (it has no login)")
	defaultUI := "none"
	if hasDisplay() {
		defaultUI = "window"
	}
	ui := fs.String("ui", defaultUI, "how to show the web interface: window (a window of its own; closing it stops darts-o-cam), browser, or none (default none without a display)")
	verbose := fs.Bool("v", false, "log whenever the board starts moving")
	saveDir := fs.String("save", "", "directory to keep the camera frames of every detection in as PNG, for debugging (default: only the last ones, in memory)")
	pprofAddr := fs.String("pprof", "", profiling.Usage)
	_ = fs.Parse(args)
	if !slices.Contains([]string{"window", "browser", "none"}, *ui) {
		return fmt.Errorf("-ui must be window, browser or none, not %q", *ui)
	}
	if *ui == "window" {
		// Started as an app (see tools/macapp) there is no terminal to show errors in.
		defer func() {
			if err != nil {
				showError(err)
			}
		}()
	}

	ln, err := net.Listen("tcp", *addr)
	if errors.Is(err, syscall.EADDRINUSE) {
		return fmt.Errorf("%s is already in use: is darts-o-cam running already?", *addr)
	}
	if err != nil {
		return fmt.Errorf("web interface: %w", err)
	}
	log.Printf("configuration in %s", *dir)
	app, err := camapp.New(*dir, camapp.Options{SaveDir: *saveDir, Verbose: *verbose})
	if err != nil {
		return err
	}
	defer app.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	update.Start(ctx)
	defer stop()
	if err := profiling.Start(ctx, *pprofAddr); err != nil {
		return fmt.Errorf("profiling: %w", err)
	}
	// Requests end with the program, also the overview's open stream of detections.
	srv := &http.Server{Handler: camui.New(app).Routes(), BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	url := "http://" + browserHost(ln.Addr())
	log.Printf("web interface on %s (stop with Ctrl+C)", url)
	switch *ui {
	case "window":
		// The window runs here, on the main goroutine; closing it stops darts-o-cam.
		if err := showWindow(ctx, url); err != nil {
			log.Printf("no window of its own (%v), opening the browser instead", err)
			openOrLog(url)
		} else {
			stop()
		}
	case "browser":
		openOrLog(url)
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func openOrLog(url string) {
	if err := openBrowser(url); err != nil {
		log.Printf("open the browser: %v", err)
	}
}

// browserHost returns the address to open in a browser: a server listening on all
// interfaces or on the loopback interface is reachable on this computer as localhost.
func browserHost(addr net.Addr) string {
	tcp, ok := addr.(*net.TCPAddr)
	if !ok || tcp.IP.IsUnspecified() || tcp.IP.IsLoopback() {
		_, port, _ := net.SplitHostPort(addr.String())
		return net.JoinHostPort("localhost", port)
	}
	return addr.String()
}

// analyze runs the detection on saved events again and draws what each camera saw:
// the changed area in red and the detected tip as a cross.
func analyze(args []string) error {
	fs := flag.NewFlagSet("analyze", flag.ExitOnError)
	defaultDir, err := camapp.DefaultDir()
	if err != nil {
		return err
	}
	configDir := fs.String("config", defaultDir, "directory with the configuration of the cameras")
	dir := fs.String("dir", ".", "directory with the frames saved with -save")
	out := fs.String("out", "", "directory for the annotated images (default: -dir)")
	_ = fs.Parse(args)
	if *out == "" {
		*out = *dir
	}

	cfg, err := camapp.LoadConfig(*configDir)
	if err != nil {
		return err
	}
	calib, watch := cfg.Calibration(*configDir), cfg.Boards[0].Detection // of the first board
	opts := watch.Detect
	events, err := savedEvents(*dir)
	if err != nil {
		return err
	}
	// The images of the empty board, to tell darts pulled out like the detection does.
	empty := map[string]*image.RGBA{}
	for _, cam := range calib.Cameras {
		if img, err := calib.LoadEmpty(cam); err == nil {
			empty[cam.Name] = img
		}
	}
	// The changed pixels of the darts found per camera since the last takeout, like the
	// detection remembers them: a dart moved by the next one is left out then.
	earlier := map[string][]image.Point{}
	for _, n := range events {
		type frames struct {
			cam           vision.CameraCalibration
			before, after *image.RGBA
			area          []image.Point
		}
		var cams []frames
		// Like the detection: only if every camera with frames has an image of the empty
		// board (a camera that delivered none, e.g. one left out, doesn't count).
		withEmpty := true
		takeout := false // a change too large for a dart, like the detection tells it first
		for _, cam := range calib.Cameras {
			prefix := filepath.Join(*dir, fmt.Sprintf("%03d-%s", n, cam.Name))
			before, err1 := readFrame(prefix + "-before")
			after, err2 := readFrame(prefix + "-after")
			if err1 != nil || err2 != nil {
				continue
			}
			m, err := cam.Mapping()
			if err != nil {
				return err
			}
			scale := vision.PixelScale(m.ToBoard)
			cams = append(cams, frames{cam, before, after, vision.ChangedArea(before, after, opts, scale)})
			takeout = takeout || watch.Takeout(before, after, scale)
			withEmpty = withEmpty && empty[cam.Name] != nil
		}
		withEmpty = withEmpty && len(cams) > 0
		if !takeout && withEmpty {
			var before, after, empties []*image.RGBA
			var areas [][]image.Point
			for _, c := range cams {
				before, after, areas = append(before, c.before), append(after, c.after), append(areas, c.area)
				empties = append(empties, empty[c.cam.Name])
			}
			takeout = vision.PulledOut(before, after, empties, areas)
		}
		if takeout {
			fmt.Printf("event %d: darts pulled out\n", n)
			clear(earlier)
			continue
		}

		var tips []vision.Tip
		for _, c := range cams {
			m, _ := c.cam.Mapping()
			scale := vision.PixelScale(m.ToBoard)
			annotated := image.NewRGBA(c.after.Bounds())
			copy(annotated.Pix, c.after.Pix)
			for _, p := range c.area {
				annotated.SetRGBA(p.X, p.Y, color.RGBA{255, 0, 0, 255})
			}

			line := fmt.Sprintf("event %d %s: changed %d px", n, c.cam.Name, len(c.area))
			if d, dartArea, err := vision.DetectDartBeside(c.before, c.after, opts, scale, earlier[c.cam.Name]); err != nil {
				line += ", " + err.Error()
			} else {
				earlier[c.cam.Name] = append(earlier[c.cam.Name], dartArea...)
				line += fmt.Sprintf(", dart %d px", len(dartArea))
				b := m.Tip(d, opts.TipShift)
				radial, tangential := m.Resolution(b)
				line += fmt.Sprintf(", tip %.0f,%.0f px -> %.0f,%.0f mm (%.1f mm from the center) %s (%.1f/%.1f px/mm towards the center/along the ring)", d.Tip.X, d.Tip.Y, b.X, b.Y, b.Radius(), dartboard.Score(b).String(), radial, tangential)
				if d.Hidden {
					line += ", ends on an earlier dart (may be hidden)"
				}
				if b.Radius() > vision.MaxBoardRadius {
					line += " (off the board, ignored)"
				} else {
					tips = append(tips, vision.Tip{Point: b, Radial: radial, Tangential: tangential, Hidden: d.Hidden})
				}
				vision.DrawCross(annotated, d.Tip, 8, color.RGBA{0, 255, 0, 255})
			}
			fmt.Println(line)
			if err := camapp.WriteJPEG(filepath.Join(*out, fmt.Sprintf("%03d-%s-analyzed.jpg", n, c.cam.Name)), annotated); err != nil {
				return err
			}
		}
		switch p, err := vision.FuseTips(tips); {
		case err == nil:
			fmt.Printf("event %d: %s at %.0f,%.0f mm (%.1f mm from the center)\n", n, dartboard.Score(p).String(), p.X, p.Y, p.Radius())
		case withEmpty:
			fmt.Printf("event %d: a miss no camera located (nothing was pulled out)\n", n)
		default:
			clear(earlier) // no dart anywhere: most likely the darts were pulled out
		}
	}
	if len(events) == 0 {
		return fmt.Errorf("no saved frames (like 001-cam1-before.png) in %s", *dir)
	}
	return nil
}

// frameExts are the formats of saved frames: PNG from -save, JPEG downloaded from the
// event page.
var frameExts = []string{".png", ".jpg"}

// savedEvents returns the numbers of the events with frames in dir, e.g. 30 for
// 030-cam1-before.png, in order.
func savedEvents(dir string) ([]int, error) {
	var events []int
	for _, ext := range frameExts {
		files, err := filepath.Glob(filepath.Join(dir, "*-before"+ext))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			var n int
			if _, err := fmt.Sscanf(filepath.Base(f), "%d-", &n); err == nil && !slices.Contains(events, n) {
				events = append(events, n)
			}
		}
	}
	slices.Sort(events)
	return events, nil
}

// readFrame reads a saved frame, path without the extension: the PNG if there is one,
// else the JPEG.
func readFrame(path string) (*image.RGBA, error) {
	var err error
	for _, ext := range frameExts {
		var img *image.RGBA
		if img, err = readImage(path + ext); err == nil {
			return img, nil
		}
	}
	return nil, err
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
