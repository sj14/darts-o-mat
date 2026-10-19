# The server darts-o-mat as a container image (darts-o-cam needs the cameras, see
# cam/TODO.md). From the project folder:
#
#	docker build -f Containerfile -t darts-o-mat .
#	docker run -p 3000:3000 -v darts-o-mat:/data darts-o-mat
#
# Without cgo (SQLite is pure Go), so the binary needs nothing from the image: distroless
# static, as the user nonroot. Built on the build machine's platform for the target one,
# so multi-platform images (--platform linux/amd64,linux/arm64) build without emulation.

FROM --platform=$BUILDPLATFORM golang:1 AS build
ARG TARGETOS TARGETARCH
# Shown in the page footer; the build has no git to take it from.
ARG VERSION=""
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X github.com/sj14/darts-o-mat/common/buildinfo.stamped=${VERSION}" -o /darts-o-mat ./server/cmd/darts-o-mat
# The SQLite database's folder, owned by nonroot so a new volume is writable.
RUN mkdir /data

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /darts-o-mat /darts-o-mat
COPY --from=build --chown=nonroot:nonroot /data /data
# The default database file:darts-o-mat.db is relative to the working directory, so it
# ends up in the volume; flags go after the image name, e.g. -registration=false.
WORKDIR /data
VOLUME /data
EXPOSE 3000
ENTRYPOINT ["/darts-o-mat"]
