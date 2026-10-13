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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sj14/darts-o-mat/tools/internal/appicon"
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
	goBuild := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", filepath.Join(contents, "MacOS", name), "./cmd/"+name)
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
// when asking for camera access.
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
		{"NSCameraUsageDescription", "darts-o-cam detects the darts thrown with the cameras at the board."},
	} {
		fmt.Fprintf(&b, "\t<key>%s</key>\n\t<string>", kv[0])
		_ = xml.EscapeText(&b, []byte(kv[1]))
		b.WriteString("</string>\n")
	}
	b.WriteString("\t<key>NSHighResolutionCapable</key>\n\t<true/>\n</dict>\n</plist>\n")
	return b.String()
}

// icon renders the app icon in all sizes of a macOS icon and converts them with iconutil.
func icon(icns string) error {
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
			if err := appicon.WritePNG(filepath.Join(set, file), size*scale); err != nil {
				return err
			}
		}
	}
	return run(exec.Command("iconutil", "--convert", "icns", "--output", icns, set))
}
