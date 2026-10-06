# Build the manager binary.
# --platform=$BUILDPLATFORM keeps the builder on the native build host so the Go
# toolchain cross-compiles to $TARGETARCH (see the build step below) instead of
# running under slow QEMU emulation for non-native target platforms.
FROM --platform=${BUILDPLATFORM} golang:1.27 AS builder
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Cache deps in a dedicated layer with mount cache for faster rebuilds
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/

# -trimpath for reproducible builds; -s -w strips debug info.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath \
    -ldflags="-s -w \
      -X main.version=${VERSION} \
      -X main.gitCommit=${GIT_COMMIT} \
      -X main.buildDate=${BUILD_DATE}" \
    -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="kube-drift" \
      org.opencontainers.image.description="Kubernetes operator that detects drift between desired-state manifests and the live cluster" \
      org.opencontainers.image.url="https://github.com/somaz94/kube-drift" \
      org.opencontainers.image.source="https://github.com/somaz94/kube-drift" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
