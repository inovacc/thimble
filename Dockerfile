# syntax=docker/dockerfile:1

# ── Build stage ──
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN set -ex && \
    echo "Building for GOOS=${TARGETOS} GOARCH=${TARGETARCH}" && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
    -trimpath \
    -ldflags="-s -w \
      -X 'main.Version=${VERSION}' \
      -X 'main.GitHash=$(git rev-parse --short HEAD 2>/dev/null || echo none)' \
      -X 'main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o /thimble ./cmd/thimble/ && \
    echo "Build succeeded: $(ls -lh /thimble)"

# ── Runtime stage ──
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /thimble /usr/local/bin/thimble

USER nonroot:nonroot

ENTRYPOINT ["thimble"]
