# Builds the apps of darts-o-cam. Everything else works with the go command alone
# (go run, go test, go generate), see README.md.

# Both apps; the macOS app only on macOS.
.PHONY: apps
apps: linux $(if $(filter Darwin,$(shell uname -s)),macapp)

# darts-o-cam.app (macOS only, see tools/macapp).
.PHONY: macapp 
macapp:
	go run ./tools/macapp

# .deb, .rpm and .tar.gz for amd64 and arm64 in dist/ (see tools/linuxpkg, a module of
# its own). The version comes from the latest vX.Y.Z tag; set one with VERSION=1.2.3.
.PHONY: linux 
linux:
	go -C tools/linuxpkg run . $(if $(VERSION),-version $(VERSION))

.PHONY: clean
clean:
	rm -rf darts-o-cam.app dist
