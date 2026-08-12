# Build stage. Runs on the builder's own arch and cross-compiles, so an
# arm64 machine building for Cloud Run does not pay for emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.1.0
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -ldflags "-X main.version=${VERSION}" -o /out/server ./cmd/server

# Runtime: distroless static (no shell). Nonroot UID 65532.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/server /server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/server"]
