// Command macapp builds darts-o-cam as a macOS app bundle:
//
//	go run ./tools/macapp            # darts-o-cam.app in the current directory
//	go run ./tools/macapp -o ~/Applications/darts-o-cam.app
//
// Started from the bundle (double click or `open`), macOS treats darts-o-cam as an app
// of its own: it asks for camera access for "darts-o-cam" with the explanation in
// Info.plist, instead of for the terminal that started it, and shows it in the Dock.
// The bundle is signed ad hoc (no developer account needed); macOS ties the camera
// access to the signature, so after a rebuild it may ask again.
//
// Runs on macOS only (it uses iconutil and codesign) and builds for its architecture.
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/sj14/darts-o-mat/pkg/frontend/static"
)

const (
	name     = "darts-o-cam"
	bundleID = "com.github.sj14.darts-o-cam"
)

func main() {
	log.SetFlags(0)
	out := flag.String("o", name+".app", "path of the app bundle to create (replaced if it exists)")
	flag.Parse()
	if runtime.GOOS != "darwin" {
		log.Fatal("macapp: builds a macOS app bundle and needs macOS (iconutil, codesign)")
	}
	if err := build(*out); err != nil {
		log.Fatalf("macapp: %v", err)
	}
	log.Printf("built %s: start it with a double click or `open %s`", *out, *out)
}

func build(app string) error {
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	contents := filepath.Join(app, "Contents")
	for _, dir := range []string{"MacOS", "Resources"} {
		if err := os.MkdirAll(filepath.Join(contents, dir), 0o755); err != nil {
			return err
		}
	}

	// No cgo: the window's web view is loaded at runtime (see cmd/darts-o-cam/window.go).
	goBuild := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(contents, "MacOS", name), "./cmd/"+name)
	goBuild.Env = append(os.Environ(), "CGO_ENABLED=0")
	if err := run(goBuild); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(infoPlist(version())), 0o644); err != nil {
		return err
	}
	if err := icon(filepath.Join(contents, "Resources", "AppIcon.icns")); err != nil {
		return err
	}
	return run(exec.Command("codesign", "--force", "--sign", "-", "--identifier", bundleID, app))
}

func run(cmd *exec.Cmd) error {
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
	}
	return nil
}

// version is the git description of the checkout, e.g. v0.1.0-3-gabc1234, or "dev".
func version() string {
	v, err := exec.Command("git", "describe", "--tags", "--always", "--dirty").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(v))
}

// infoPlist describes the bundle. NSCameraUsageDescription is the explanation macOS shows
// when asking for camera access (in German, like the user interface).
func infoPlist(version string) string {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	for _, kv := range [][2]string{
		{"CFBundleName", name},
		{"CFBundleDisplayName", name},
		{"CFBundleIdentifier", bundleID},
		{"CFBundleExecutable", name},
		{"CFBundlePackageType", "APPL"},
		{"CFBundleIconFile", "AppIcon"},
		{"CFBundleShortVersionString", version},
		{"CFBundleVersion", version},
		{"NSCameraUsageDescription", "darts-o-cam erkennt mit den Kameras am Board die geworfenen Pfeile."},
	} {
		fmt.Fprintf(&b, "\t<key>%s</key>\n\t<string>", kv[0])
		_ = xml.EscapeText(&b, []byte(kv[1]))
		b.WriteString("</string>\n")
	}
	b.WriteString("\t<key>NSHighResolutionCapable</key>\n\t<true/>\n</dict>\n</plist>\n")
	return b.String()
}

// icon renders the favicon's rings in all sizes of a macOS icon and converts them with iconutil.
func icon(icns string) error {
	rings, err := faviconRings()
	if err != nil {
		return err
	}
	set, err := os.MkdirTemp("", "AppIcon-*.iconset")
	if err != nil {
		return err
	}
	defer os.RemoveAll(set)
	for _, size := range []int{16, 32, 128, 256, 512} {
		for _, scale := range []int{1, 2} {
			file := fmt.Sprintf("icon_%dx%d.png", size, size)
			if scale == 2 {
				file = fmt.Sprintf("icon_%dx%d@2x.png", size, size)
			}
			if err := writePNG(filepath.Join(set, file), renderRings(rings, size*scale)); err != nil {
				return err
			}
		}
	}
	return run(exec.Command("iconutil", "--convert", "icns", "--output", icns, set))
}

// ring is a filled circle of the favicon, in its 32×32 view box.
type ring struct {
	r   float64
	col color.RGBA
}

// faviconRings reads the concentric circles of favicon.svg, outermost first.
func faviconRings() ([]ring, error) {
	data, err := static.FS.ReadFile("favicon.svg")
	if err != nil {
		return nil, err
	}
	var svg struct {
		Circles []struct {
			R    string `xml:"r,attr"`
			Fill string `xml:"fill,attr"`
		} `xml:"circle"`
	}
	if err := xml.Unmarshal(data, &svg); err != nil {
		return nil, err
	}
	var rings []ring
	for _, c := range svg.Circles {
		r, err := strconv.ParseFloat(c.R, 64)
		if err != nil {
			return nil, fmt.Errorf("favicon.svg: radius %q: %w", c.R, err)
		}
		var col color.RGBA
		if _, err := fmt.Sscanf(c.Fill, "#%02x%02x%02x", &col.R, &col.G, &col.B); err != nil {
			return nil, fmt.Errorf("favicon.svg: color %q: %w", c.Fill, err)
		}
		col.A = 255
		rings = append(rings, ring{r, col})
	}
	return rings, nil
}

// renderRings draws the rings into a square image. Like other macOS icons, the disc
// leaves a margin of about a tenth on each side. Edges are smoothed by sampling each
// pixel 4×4 times.
func renderRings(rings []ring, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	const n = 4
	center := float64(size) / 2
	scale := float64(size) * 0.8 / 32 // the favicon's 32 units fill 80 % of the icon
	for y := range size {
		for x := range size {
			var r, g, b, hits float64
			for sy := range n {
				for sx := range n {
					dx := (float64(x) + (float64(sx)+0.5)/n - center) / scale
					dy := (float64(y) + (float64(sy)+0.5)/n - center) / scale
					d := dx*dx + dy*dy
					// The innermost ring containing the sample wins.
					for _, rg := range slices.Backward(rings) {
						if d <= rg.r*rg.r {
							c := rg.col
							r, g, b, hits = r+float64(c.R), g+float64(c.G), b+float64(c.B), hits+1
							break
						}
					}
				}
			}
			if hits > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(r / hits), uint8(g / hits), uint8(b / hits), uint8(255 * hits / (n * n))})
			}
		}
	}
	return img
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
