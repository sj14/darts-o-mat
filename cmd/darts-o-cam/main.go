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

	"github.com/sj14/darts-o-mat/pkg/camapp"
	"github.com/sj14/darts-o-mat/pkg/camui"
	"github.com/sj14/darts-o-mat/pkg/domain/dartboard"
	"github.com/sj14/darts-o-mat/pkg/vision"
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
func run(args []string) error {
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
	open := fs.Bool("open", hasDisplay(), "open the web interface in the browser; off by default without a display")
	verbose := fs.Bool("v", false, "log whenever the board starts moving")
	saveDir := fs.String("save", "", "directory to keep the camera frames of every detection in, for debugging (default: only the last ones, in the cache directory)")
	_ = fs.Parse(args)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("web interface: %w", err)
	}
	log.Printf("configuration in %s", *dir)
	opts := camapp.Options{FrameDir: *saveDir, KeepFrames: *saveDir != "", Verbose: *verbose}
	if *saveDir == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return err
		}
		opts.FrameDir = filepath.Join(cache, "darts-o-cam", "events")
	}
	app, err := camapp.New(*dir, opts)
	if err != nil {
		return err
	}
	defer app.Close()

	srv := &http.Server{Handler: camui.New(app).Routes()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	url := "http://" + browserHost(ln.Addr())
	log.Printf("web interface on %s (stop with Ctrl+C)", url)
	if *open {
		if err := openBrowser(url); err != nil {
			log.Printf("open the browser: %v", err)
		}
	}
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
	calib, opts := cfg.Calibration(*configDir), cfg.Detection.Detect
	events, err := savedEvents(*dir)
	if err != nil {
		return err
	}
	for _, n := range events {
		var points []dartboard.Point
		for _, cam := range calib.Cameras {
			prefix := filepath.Join(*dir, fmt.Sprintf("%03d-%s", n, cam.Name))
			before, err1 := readImage(prefix + "-before.jpg")
			after, err2 := readImage(prefix + "-after.jpg")
			if err1 != nil || err2 != nil {
				continue
			}

			m, err := cam.Mapping()
			if err != nil {
				return err
			}
			scale := vision.PixelScale(m.ToBoard)
			area := vision.ChangedArea(before, after, opts, scale)
			annotated := image.NewRGBA(after.Bounds())
			copy(annotated.Pix, after.Pix)
			for _, p := range area {
				annotated.SetRGBA(p.X, p.Y, color.RGBA{255, 0, 0, 255})
			}

			line := fmt.Sprintf("event %d %s: changed %d px", n, cam.Name, len(area))
			if d, err := vision.DetectDart(before, after, opts, scale); err != nil {
				line += ", " + err.Error()
			} else {
				b := m.Tip(d, opts.TipShift)
				line += fmt.Sprintf(", tip %.0f,%.0f px -> %.0f,%.0f mm %s", d.Tip.X, d.Tip.Y, b.X, b.Y, dartboard.Score(b).String())
				if b.Radius() > vision.MaxBoardRadius {
					line += " (off the board, ignored)"
				} else {
					points = append(points, b)
				}
				vision.DrawCross(annotated, d.Tip, 8, color.RGBA{0, 255, 0, 255})
			}
			fmt.Println(line)
			if err := camapp.WriteJPEG(filepath.Join(*out, fmt.Sprintf("%03d-%s-analyzed.jpg", n, cam.Name)), annotated); err != nil {
				return err
			}
		}
		if p, err := vision.FuseTips(points); err == nil {
			fmt.Printf("event %d: %s at %.0f,%.0f mm\n", n, dartboard.Score(p).String(), p.X, p.Y)
		}
	}
	if len(events) == 0 {
		return fmt.Errorf("no saved frames (like 001-cam1-before.jpg) in %s", *dir)
	}
	return nil
}

// savedEvents returns the numbers of the events with frames in dir, e.g. 30 for
// 030-cam1-before.jpg, in order.
func savedEvents(dir string) ([]int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*-before.jpg"))
	if err != nil {
		return nil, err
	}
	var events []int
	for _, f := range files {
		var n int
		if _, err := fmt.Sscanf(filepath.Base(f), "%d-", &n); err == nil && !slices.Contains(events, n) {
			events = append(events, n)
		}
	}
	slices.Sort(events)
	return events, nil
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
