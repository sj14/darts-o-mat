// Command tailwind runs the Tailwind CSS standalone CLI with the given arguments.
// It downloads the pinned version once into the user's cache directory and checks its
// checksum, so building the CSS needs neither Node nor a manual install:
//
//	go run ./tools/tailwind -i input.css -o tailwind.css --minify
//
// Only needed when styles change; the generated CSS is committed (see go:generate in
// pkg/frontend/static).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const version = "v4.3.3"

// checksums of the release binaries, from the GitHub release.
var checksums = map[string]string{
	"linux-arm64":     "55fd0b241214eff3de1e8ee4f22796662f2d2e7a49bcfca7477cfd0bac398195",
	"linux-x64":       "dc61b3ac6b8c9ca874c0cc4c57b2409791a64c5540404ca5f5367360babc313a",
	"macos-arm64":     "cdf646702987a743464dff4d9c60fd4480d1c1e73dd819a9a67f1078815dce9d",
	"macos-x64":       "7922e0953f2110c05976e3bf58f14e643d90427575e766b7d433f5f80cbee7e1",
	"windows-x64.exe": "e0e260ce048014e9268f6237ff18f8ccf02cef521cbd0ae04e82c2cdf7aa3955",
}

func main() {
	log.SetFlags(0)
	bin, err := binary()
	if err != nil {
		log.Fatalf("tailwind: %v", err)
	}
	cmd := exec.Command(bin, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("tailwind: %v", err)
	}
}

// binary returns the path of the CLI, downloading it if needed.
func binary() (string, error) {
	platform, err := platformName()
	if err != nil {
		return "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cache, "darts-o-mat", "tailwindcss-"+version+"-"+platform)
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}

	url := "https://github.com/tailwindlabs/tailwindcss/releases/download/" + version + "/tailwindcss-" + platform
	log.Printf("downloading %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "download-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, hash), resp.Body); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != checksums[platform] {
		return "", fmt.Errorf("checksum of %s is %s, want %s", url, got, checksums[platform])
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

func platformName() (string, error) {
	osName := map[string]string{"linux": "linux", "darwin": "macos", "windows": "windows"}[runtime.GOOS]
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	name := osName + "-" + arch
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if _, ok := checksums[name]; !ok {
		return "", fmt.Errorf("no Tailwind CLI for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return name, nil
}
