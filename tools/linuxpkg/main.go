// Command linuxpkg builds the Linux packages of darts-o-cam and of the server darts-o-mat,
// for amd64 and arm64 (e.g. a Raspberry Pi):
//
//	go -C tools/linuxpkg run .                     # into dist/ of the project
//	go -C tools/linuxpkg run . -o /tmp/pkgs -version 1.2.3
//
// It is part of the tools module, so nFPM and its dependencies stay out of the project's go.mod.
//
// darts-o-cam gets per architecture a .deb, an .rpm and a .tar.gz, each with the program,
// a menu entry (darts-o-cam.desktop), icons and a systemd service for running it without
// a desktop (darts-o-cam.service, not enabled). It is built without cgo; WebKitGTK, for
// its window of its own, is only recommended: without it the program opens the browser.
//
// darts-o-mat gets the same three, with the program, a systemd service
// (darts-o-mat.service, not enabled; the games are kept in /var/lib/darts-o-mat), LICENSE
// and README in /usr/share/doc/darts-o-mat. So every Linux package of both programs comes
// from here and has the same layout; the server's macOS and Windows archives are built by
// the release workflow.
//
// The files have no version in their names (darts-o-cam_linux_amd64.deb, .rpm, .tar.gz),
// so the links to the latest release's downloads stay the same; the version is in the
// packages and in the folder inside the .tar.gz.
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
	"strings"
	"time"

	"github.com/goreleaser/nfpm/v2"
	_ "github.com/goreleaser/nfpm/v2/deb" // registers the packagers
	"github.com/goreleaser/nfpm/v2/files"
	_ "github.com/goreleaser/nfpm/v2/rpm"

	"github.com/sj14/darts-o-mat/tools/internal/appicon"
	"github.com/sj14/darts-o-mat/tools/internal/project"
	"github.com/sj14/darts-o-mat/tools/internal/version"
)

var (
	//go:embed darts-o-cam.desktop
	desktopEntry []byte
	//go:embed darts-o-cam.service
	cameraUnit []byte
	//go:embed darts-o-mat.service
	serverUnit []byte
)

// iconSizes are the sizes of the icons in /usr/share/icons/hicolor.
var iconSizes = []int{16, 32, 48, 64, 128, 256, 512}

// asset is a file of a package besides the program, the same for every architecture. Its
// content is data, or else the file of the project it names (relative to the project).
type asset struct {
	data []byte
	file string
	dst  string
}

// program is one of the programs that get packages.
type program struct {
	name        string
	description string
	assets      []asset
	// icons adds the program's icon in every size of iconSizes.
	icons bool
	// stamp sets the version in the binary. Without it the binary tells the git revision it
	// was built from, which for a release build is not the version.
	stamp bool
	// formats are the packages written: "deb", "rpm", and "tar" for a .tar.gz.
	formats []string
	// The packages' dependencies, per format.
	recommends map[string][]string
}

// loaders are glibc's dynamic loaders per architecture, written into the binaries:
// purego links darts-o-cam dynamically even with CGO_ENABLED=0, and the Go of NixOS
// writes the path of its own loader into binaries for its architecture, which no
// other system has.
var loaders = map[string]string{
	"amd64": "/lib64/ld-linux-x86-64.so.2",
	"arm64": "/lib/ld-linux-aarch64.so.1",
}

var programs = []program{
	{
		name:        "darts-o-cam",
		description: "Detects darts with the cameras at a dartboard and reports them to darts-o-mat.",
		assets: []asset{
			{data: desktopEntry, dst: "/usr/share/applications/darts-o-cam.desktop"},
			{data: cameraUnit, dst: "/usr/lib/systemd/system/darts-o-cam.service"},
		},
		icons:   true,
		formats: []string{"deb", "rpm", "tar"},
		// WebKitGTK only for the window: GTK4 with WebKitGTK 6.0 first, else GTK3 with
		// WebKit2GTK 4.1 or 4.0 (glaze takes what's there). libjpeg-turbo makes decoding
		// the camera frames far cheaper (internal/cam/camera); Debian calls its package
		// libturbojpeg0, Ubuntu libturbojpeg.
		recommends: map[string][]string{
			"deb": {
				"libwebkitgtk-6.0-4 | libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37",
				"libturbojpeg0 | libturbojpeg",
			},
			"rpm": {"(webkitgtk6.0 or webkit2gtk4.1 or webkit2gtk4.0)", "turbojpeg"},
		},
	},
	{
		name:        "darts-o-mat",
		description: "A free, self-hosted darts scorer: enter the throws or let cameras detect them.",
		assets: []asset{
			{data: serverUnit, dst: "/usr/lib/systemd/system/darts-o-mat.service"},
			{file: "LICENSE", dst: "/usr/share/doc/darts-o-mat/LICENSE"},
			{file: "cmd/darts-o-mat/README.md", dst: "/usr/share/doc/darts-o-mat/README.md"},
		},
		stamp:   true,
		formats: []string{"deb", "rpm", "tar"},
	},
}

func main() {
	log.SetFlags(0)
	root, err := project.Dir()
	if err != nil {
		log.Fatalf("linuxpkg: %v", err)
	}
	out := flag.String("o", filepath.Join(root, "dist"), "directory for the packages")
	ver := flag.String("version", version.FromGit(root), "version of the packages")
	flag.Parse()
	if err := build(root, *out, strings.TrimPrefix(*ver, "v")); err != nil {
		log.Fatalf("linuxpkg: %v", err)
	}
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
	for _, p := range programs {
		if err := p.build(root, out, work, version); err != nil {
			return fmt.Errorf("%s: %w", p.name, err)
		}
	}
	return nil
}

// build writes the packages of the program for every architecture.
func (p program) build(root, out, work, version string) error {
	// The files besides the program, the same for every architecture.
	var assets files.Contents
	for _, a := range p.assets {
		data := a.data
		if a.file != "" {
			var err error
			if data, err = os.ReadFile(filepath.Join(root, a.file)); err != nil {
				return err
			}
		}
		src := filepath.Join(work, p.name, filepath.Base(a.dst))
		if err := writeFile(src, data); err != nil {
			return err
		}
		assets = append(assets, &files.Content{Source: src, Destination: a.dst, FileInfo: &files.ContentFileInfo{Mode: 0o644}})
	}
	if p.icons {
		for _, size := range iconSizes {
			src := filepath.Join(work, p.name, fmt.Sprintf("icon-%d.png", size))
			if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
				return err
			}
			if err := appicon.WritePNG(src, size); err != nil {
				return err
			}
			dst := fmt.Sprintf("/usr/share/icons/hicolor/%dx%d/apps/%s.png", size, size, p.name)
			assets = append(assets, &files.Content{Source: src, Destination: dst, FileInfo: &files.ContentFileInfo{Mode: 0o644}})
		}
	}

	ldflags := "-s -w"
	if p.stamp {
		ldflags += " -X github.com/sj14/darts-o-mat/internal/common/buildinfo.stamped=" + version
	}
	for _, arch := range []string{"amd64", "arm64"} {
		bin := filepath.Join(work, p.name, arch, p.name)
		archLdflags := ldflags + " -I " + loaders[arch]
		goBuild := exec.Command("go", "build", "-trimpath", "-ldflags="+archLdflags, "-o", bin, "./cmd/"+p.name)
		goBuild.Dir = root
		goBuild.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+arch)
		goBuild.Stdout, goBuild.Stderr = os.Stdout, os.Stderr
		if err := goBuild.Run(); err != nil {
			return fmt.Errorf("build for %s: %w", arch, err)
		}
		program := &files.Content{Source: bin, Destination: "/usr/bin/" + p.name, FileInfo: &files.ContentFileInfo{Mode: 0o755}}
		contents := append(files.Contents{program}, assets...)
		for _, format := range p.formats {
			var err error
			if format == "tar" {
				err = p.tarball(out, arch, version, contents)
			} else {
				err = p.pkg(out, format, arch, version, contents)
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// pkg writes a .deb or .rpm package.
func (p program) pkg(out, format, arch, version string, contents files.Contents) error {
	info := nfpm.WithDefaults(&nfpm.Info{
		Name:          p.name,
		Arch:          arch,
		Version:       version,
		VersionSchema: "none", // as given, e.g. 0.0.0+git.abc1234
		Maintainer:    "darts-o-mat <https://github.com/sj14/darts-o-mat>",
		Description:   p.description,
		Homepage:      "https://github.com/sj14/darts-o-mat",
		Section:       "games",
		MTime:         time.Now(),
		Overridables: nfpm.Overridables{
			Contents:   contents,
			Recommends: p.recommends[format],
		},
	})
	if err := nfpm.PrepareForPackager(info, format); err != nil {
		return err
	}
	packager, err := nfpm.Get(format)
	if err != nil {
		return err
	}
	// No version in the name (darts-o-cam_linux_amd64.deb): a link to the latest release's
	// download stays the same. The version is in the package.
	path := filepath.Join(out, fmt.Sprintf("%s_linux_%s.%s", p.name, arch, format))
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

// tarball writes the same files as the packages into darts-o-cam_linux_<arch>.tar.gz,
// below a folder darts-o-cam_<version>_linux_<arch> (usr/bin/…, usr/share/…), to unpack
// anywhere.
func (p program) tarball(out, arch, version string, contents files.Contents) error {
	dir := fmt.Sprintf("%s_%s_linux_%s", p.name, version, arch)
	path := filepath.Join(out, fmt.Sprintf("%s_linux_%s.tar.gz", p.name, arch))
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
