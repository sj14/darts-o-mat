// Command linuxpkg builds the Linux packages of darts-o-cam, for amd64 and arm64
// (e.g. a Raspberry Pi):
//
//	go -C tools/linuxpkg run .                     # into dist/ of the project
//	go -C tools/linuxpkg run . -o /tmp/pkgs -version 1.2.3
//
// It is a module of its own, so nFPM and its dependencies stay out of the project's go.mod.
//
// Per architecture a .deb, an .rpm and a .tar.gz, each with the program, a menu entry
// (darts-o-cam.desktop), icons and a systemd service for running it without a desktop
// (darts-o-cam.service, not enabled). darts-o-cam is built without cgo; WebKitGTK, for
// its window of its own, is only recommended: without it the program opens the browser.
// .deb and .rpm are written with nFPM, so neither dpkg nor rpmbuild is needed; it runs
// on any system.
package main

import (
	"archive/tar"
	"compress/gzip"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/deb" // registers the packagers
	"github.com/goreleaser/nfpm/v2/files"
	_ "github.com/goreleaser/nfpm/v2/rpm"

	"github.com/sj14/darts-o-mat/tools/internal/appicon"
)

const name = "darts-o-cam"

var (
	//go:embed darts-o-cam.desktop
	desktopEntry []byte
	//go:embed darts-o-cam.service
	serviceUnit []byte
)

// iconSizes are the sizes of the icons in /usr/share/icons/hicolor.
var iconSizes = []int{16, 32, 48, 64, 128, 256, 512}

// The packages' dependencies. WebKitGTK only for the window: GTK4 with WebKitGTK 6.0
// first, else GTK3 with WebKit2GTK 4.1 or 4.0 (glaze takes what's there).
// libjpeg-turbo makes decoding the camera frames cheaper (pkg/camera).
var recommends = map[string][]string{
	"deb": {"libwebkitgtk-6.0-4 | libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37"},
	"rpm": {"(webkitgtk6.0 or webkit2gtk4.1 or webkit2gtk4.0)"},
}
var suggests = map[string][]string{
	"deb": {"libturbojpeg0"},
	"rpm": {"turbojpeg"},
}

func main() {
	log.SetFlags(0)
	root, err := projectDir()
	if err != nil {
		log.Fatalf("linuxpkg: %v", err)
	}
	out := flag.String("o", filepath.Join(root, "dist"), "directory for the packages")
	version := flag.String("version", gitVersion(root), "version of the packages")
	flag.Parse()
	if err := build(root, *out, strings.TrimPrefix(*version, "v")); err != nil {
		log.Fatalf("linuxpkg: %v", err)
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

func build(root, out, version string) error {
	work, err := os.MkdirTemp("", "linuxpkg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	// The files besides the program, the same for every architecture.
	var assets files.Contents
	for _, a := range []struct {
		data []byte
		dst  string
	}{
		{desktopEntry, "/usr/share/applications/" + name + ".desktop"},
		{serviceUnit, "/usr/lib/systemd/system/" + name + ".service"},
	} {
		src := filepath.Join(work, filepath.Base(a.dst))
		if err := os.WriteFile(src, a.data, 0o644); err != nil {
			return err
		}
		assets = append(assets, &files.Content{Source: src, Destination: a.dst, FileInfo: &files.ContentFileInfo{Mode: 0o644}})
	}
	for _, size := range iconSizes {
		src := filepath.Join(work, fmt.Sprintf("icon-%d.png", size))
		if err := appicon.WritePNG(src, size); err != nil {
			return err
		}
		dst := fmt.Sprintf("/usr/share/icons/hicolor/%dx%d/apps/%s.png", size, size, name)
		assets = append(assets, &files.Content{Source: src, Destination: dst, FileInfo: &files.ContentFileInfo{Mode: 0o644}})
	}

	for _, arch := range []string{"amd64", "arm64"} {
		bin := filepath.Join(work, arch, name)
		goBuild := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", bin, "./cmd/"+name)
		goBuild.Dir = root
		goBuild.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
		goBuild.Stdout, goBuild.Stderr = os.Stdout, os.Stderr
		if err := goBuild.Run(); err != nil {
			return fmt.Errorf("build for %s: %w", arch, err)
		}
		program := &files.Content{Source: bin, Destination: "/usr/bin/" + name, FileInfo: &files.ContentFileInfo{Mode: 0o755}}
		contents := append(files.Contents{program}, assets...)
		for _, format := range []string{"deb", "rpm"} {
			if err := pkg(out, format, arch, version, contents); err != nil {
				return err
			}
		}
		if err := tarball(out, arch, version, contents); err != nil {
			return err
		}
	}
	return nil
}

// pkg writes a .deb or .rpm package.
func pkg(out, format, arch, version string, contents files.Contents) error {
	info := nfpm.WithDefaults(&nfpm.Info{
		Name:          name,
		Arch:          arch,
		Version:       version,
		VersionSchema: "none", // as given, e.g. 0.0.0+git.abc1234
		Maintainer:    "darts-o-mat <https://github.com/sj14/darts-o-mat>",
		Description:   "Detects darts with the cameras at a dartboard and reports them to darts-o-mat.",
		Homepage:      "https://github.com/sj14/darts-o-mat",
		Section:       "games",
		MTime:         time.Now(),
		Overridables: nfpm.Overridables{
			Contents:   contents,
			Recommends: recommends[format],
			Suggests:   suggests[format],
		},
	})
	if err := nfpm.PrepareForPackager(info, format); err != nil {
		return err
	}
	packager, err := nfpm.Get(format)
	if err != nil {
		return err
	}
	path := filepath.Join(out, packager.ConventionalFileName(info))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := packager.Package(info, f); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	log.Printf("built %s", path)
	return f.Close()
}

// tarball writes the same files as the packages into a .tar.gz, below a folder
// darts-o-cam_<version>_linux_<arch> (usr/bin/…, usr/share/…), to unpack anywhere.
func tarball(out, arch, version string, contents files.Contents) error {
	dir := fmt.Sprintf("%s_%s_linux_%s", name, version, arch)
	path := filepath.Join(out, dir+".tar.gz")
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, c := range contents {
		if err := addFile(tw, c.Source, dir+c.Destination, c.FileInfo.Mode); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	log.Printf("built %s", path)
	return f.Close()
}

func addFile(tw *tar.Writer, src, dst string, mode os.FileMode) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Name: dst, Mode: int64(mode), Size: st.Size(), ModTime: st.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

// gitVersion returns the version from the latest git tag, e.g. 1.2.3 for v1.2.3. Without
// such a tag, or with commits or changes since, it is marked as a development build:
// 0.0.0+git.abc1234 (dpkg and rpm sort that before 0.0.1).
func gitVersion(root string) string {
	describe := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	describe.Dir = root
	desc, err := describe.Output()
	if err != nil {
		return "0.0.0+unknown"
	}
	d := strings.TrimSpace(string(desc))
	if m := regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`).FindStringSubmatch(d); m != nil {
		return m[1]
	}
	return "0.0.0+git." + strings.NewReplacer("-", ".").Replace(d)
}
