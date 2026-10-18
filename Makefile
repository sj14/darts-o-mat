# Builds the apps of darts-o-cam. Everything else works with the go command alone
# (go run, go test, go generate), see README.md.

# All apps; the macOS app only on macOS.
.PHONY: apps
apps: linux windows $(if $(filter Darwin,$(shell uname -s)),macapp)

# darts-o-cam.app (macOS only, see tools/macapp).
.PHONY: macapp 
macapp:
	go run ./tools/macapp

# The Linux packages for amd64 and arm64 in dist/: .deb, .rpm and .tar.gz of darts-o-cam,
# .deb and .rpm of the server (see tools/linuxpkg, a module of its own). The version comes
# from the latest vX.Y.Z tag; set one with VERSION=1.2.3.
.PHONY: linux 
linux:
	go -C tools/linuxpkg run . $(if $(VERSION),-version $(VERSION))

# A .zip with darts-o-cam.exe for amd64 and arm64 in dist/ (see tools/winpkg, a module
# of its own). Version like for linux.
.PHONY: windows
windows:
	go -C tools/winpkg run . $(if $(VERSION),-version $(VERSION))

# The project page (site/) with the fonts and icons of the app in dist/site, ready to be
# published as GitHub Pages (.github/workflows/pages.yml) or opened with a local server.
.PHONY: site
site:
	rm -rf dist/site
	mkdir -p dist/site/assets
	cp -R site/. dist/site/
	cp pkg/frontend/static/*.woff2 pkg/frontend/static/favicon.svg pkg/frontend/static/apple-touch-icon.png pkg/frontend/static/*-OFL.txt dist/site/assets/

.PHONY: clean
clean:
	rm -rf darts-o-cam.app dist
