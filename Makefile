# Builds the apps of darts-o-cam and does what needs more than one command in the
# project's several modules (generate, tidy). Everything else works with the go command
# alone (go run, go test), see README.md.

# All apps; the macOS app only on macOS.
.PHONY: apps
apps: linux windows $(if $(filter Darwin,$(shell uname -s)),macapp)

# darts-o-cam.app (macOS only, see tools/macapp).
.PHONY: macapp 
macapp:
	go run ./tools/macapp

# The Linux packages of darts-o-cam and of the server for amd64 and arm64 in dist/: .deb,
# .rpm and .tar.gz each (see tools/linuxpkg). The version comes from
# the latest vX.Y.Z tag; set one with VERSION=1.2.3.
.PHONY: linux 
linux:
	go -C tools run ./linuxpkg $(if $(VERSION),-version $(VERSION))

# A .zip with darts-o-cam.exe for amd64 and arm64 in dist/ (see tools/winpkg). Version like for linux.
.PHONY: windows
windows:
	go -C tools run ./winpkg $(if $(VERSION),-version $(VERSION))

# The project page (site/) with the fonts and icons of the app in dist/site, ready to be
# published as GitHub Pages (.github/workflows/pages.yml) or opened with a local server.
.PHONY: site
site:
	rm -rf dist/site
	mkdir -p dist/site/assets
	cp -R site/. dist/site/
	cp common/static/*.woff2 common/static/favicon.svg common/static/apple-touch-icon.png common/static/*-OFL.txt dist/site/assets/

# The generated code: the templ views of the server and of darts-o-cam, then the CSS
# (common/static, built from the classes the views use).
.PHONY: generate
generate:
	for m in server cam; do (cd $$m && go tool templ generate); done
	go generate ./common/...

# go mod tidy in every module (needs the network).
.PHONY: tidy
tidy:
	for m in common server cam tools; do (cd $$m && go mod tidy); done

.PHONY: clean
clean:
	rm -rf darts-o-cam.app dist
