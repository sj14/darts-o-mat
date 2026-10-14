// Command winpkg builds darts-o-cam for Windows, for amd64 and arm64:
//
//	go -C tools/winpkg run .                     # into dist/ of the project
//	go -C tools/winpkg run . -o /tmp/pkgs -version 1.2.3
//
// It is a module of its own, so winres stays out of the project's go.mod.
//
// Per architecture a .zip with darts-o-cam.exe and the license. The program is built
// as a GUI program (-H windowsgui), so a double click opens no console window (started
// from a terminal it logs there anyway, see cmd/darts-o-cam/console_windows.go), and
// gets the app icon, version information and a manifest (sharp on high resolution
// displays) as resources. No installer: unpack it anywhere and start the .exe. Runs on
// any system.
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tc-hib/winres"
	winversion "github.com/tc-hib/winres/version"

	"github.com/sj14/darts-o-mat/tools/internal/appicon"
	"github.com/sj14/darts-o-mat/tools/internal/version"
)

const name = "darts-o-cam"

// iconSizes are the sizes in the .exe's icon; Windows picks one per place (Explorer,
// taskbar, title bar) and display scaling.
var iconSizes = []int{16, 20, 24, 32, 40, 48, 64, 256}

func main() {
	log.SetFlags(0)
	root, err := projectDir()
	if err != nil {
		log.Fatalf("winpkg: %v", err)
	}
	out := flag.String("o", filepath.Join(root, "dist"), "directory for the packages")
	ver := flag.String("version", version.FromGit(root), "version of the packages")
	flag.Parse()
	if err := build(root, *out, strings.TrimPrefix(*ver, "v")); err != nil {
		log.Fatalf("winpkg: %v", err)
	}
}

// projectDir returns the directory of the darts-o-mat module, which this one replaces
// with ../.. (see go.mod).
func projectDir() (string, error) {
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/sj14/darts-o-mat").Output()
	if err != nil {
		return "", fmt.Errorf("find the project: %w", err)
	}
	return strings.TrimSpace(string(dir)), nil
}

func build(root, out, ver string) error {
	work, err := os.MkdirTemp("", "winpkg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	rs, err := resources(ver)
	if err != nil {
		return err
	}
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return err
	}

	for _, arch := range []string{"amd64", "arm64"} {
		bin := filepath.Join(work, arch+".exe")
		goBuild := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w -H windowsgui", "-o", bin, "./cmd/"+name)
		goBuild.Dir = root
		goBuild.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=windows", "GOARCH="+arch)
		goBuild.Stdout, goBuild.Stderr = os.Stdout, os.Stderr
		if err := goBuild.Run(); err != nil {
			return fmt.Errorf("build for %s: %w", arch, err)
		}
		exe, err := addResources(bin, rs)
		if err != nil {
			return fmt.Errorf("resources for %s: %w", arch, err)
		}
		dir := fmt.Sprintf("%s_%s_windows_%s", name, ver, arch)
		path := filepath.Join(out, dir+".zip")
		if err := writeZip(path, []zipFile{
			{dir + "/" + name + ".exe", exe},
			{dir + "/LICENSE.txt", license},
		}); err != nil {
			return err
		}
		log.Printf("built %s", path)
	}
	return nil
}

// resources returns the icon, the version information and the manifest of the .exe.
func resources(ver string) (*winres.ResourceSet, error) {
	var images []image.Image
	for _, size := range iconSizes {
		img, err := appicon.Render(size)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	icon, err := winres.NewIconFromImages(images)
	if err != nil {
		return nil, err
	}
	rs := &winres.ResourceSet{}
	if err := rs.SetIcon(winres.Name("APP"), icon); err != nil { // the first icon is the .exe's
		return nil, err
	}

	var vi winversion.Info
	for key, value := range map[string]string{
		winversion.ProductName:      name,
		winversion.FileDescription:  name, // shown in the Task Manager
		winversion.OriginalFilename: name + ".exe",
		winversion.LegalCopyright:   "AGPL-3.0, https://github.com/sj14/darts-o-mat",
	} {
		if err := vi.Set(winversion.LangNeutral, key, value); err != nil {
			return nil, err
		}
	}
	vi.SetFileVersion(ver) // the numbers in front, e.g. 0.0.0 for 0.0.0+git.abc1234
	vi.SetProductVersion(ver)
	rs.SetVersionInfo(vi)

	// Runs without admin rights and is aware of the display scaling (else Windows scales
	// the window up, blurred).
	rs.SetManifest(winres.AppManifest{
		Identity:            winres.AssemblyIdentity{Name: name},
		Compatibility:       winres.Win10AndAbove,
		UseCommonControlsV6: true,
	})
	return rs, nil
}

// addResources returns the .exe at path with the resources added.
func addResources(path string, rs *winres.ResourceSet) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var exe bytes.Buffer
	if err := rs.WriteToEXE(&exe, f); err != nil {
		return nil, err
	}
	return exe.Bytes(), nil
}

type zipFile struct {
	name string
	data []byte
}

func writeZip(path string, files []zipFile) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for _, file := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: file.name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		if _, err := w.Write(file.data); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return f.Close()
}
