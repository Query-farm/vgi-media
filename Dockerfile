# Copyright 2026 Query Farm LLC - https://query.farm
#
# Single image that serves the network transports of the `vgi-media` VGI worker:
#   docker run ... IMG            -> HTTP server on $PORT      (default; Fly.io / local)
#   docker run -i ... IMG stdio   -> stdio worker DuckDB spawns on-host
#   docker run ... IMG unix …     -> AF_UNIX launcher transport (advanced)
# See docker-entrypoint.sh.
#
# The worker EXECs `ffprobe` (from ffmpeg) as a subprocess to read media files,
# so the runtime image installs ffmpeg. It never links any ffmpeg library — the
# MIT worker only shells out to the (L)GPL ffprobe binary.
# syntax=docker/dockerfile:1

# ---- build stage -----------------------------------------------------------
# CGO is REQUIRED: the worker links DuckDB via duckdb/duckdb-go (the Arrow C
# Data Interface), so CGO_ENABLED=0 fails to build. gcc/g++ + libc headers back
# the cgo link. Native per-arch runners in CI → no cross-compilation here.
FROM golang:1.26-bookworm AS build
WORKDIR /src

ENV CGO_ENABLED=1

RUN apt-get update && apt-get install -y --no-install-recommends \
        gcc g++ libc6-dev ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Resolve modules first so the (large, CGO-linked DuckDB) dependency graph layer
# stays cached across source-only changes. BuildKit cache mounts persist the Go
# module + build caches across image rebuilds so only changed packages recompile.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" \
    -o /worker ./cmd/vgi-media-worker

# ---- runtime stage ---------------------------------------------------------
# debian-slim (not distroless) so the HEALTHCHECK has a real `curl`, and so the
# worker can EXEC the ffmpeg-provided `ffprobe` at runtime.
FROM debian:bookworm-slim

# Build metadata, wired from docker/metadata-action outputs in CI.
ARG VERSION=0.0.0
ARG GIT_COMMIT=unknown
ARG SOURCE_URL=https://github.com/Query-farm/vgi-media

# Standard OCI labels + the VGI transport-advertisement label. `transports`
# lists the NETWORK transports this image serves (http only; stdio is a spawn
# mode, not a network transport, and unix is a local launcher socket).
LABEL org.opencontainers.image.title="vgi-media" \
      org.opencontainers.image.description="Extract video/audio/container metadata via ffprobe as a VGI worker for DuckDB/SQL (stdio + HTTP)" \
      org.opencontainers.image.source="${SOURCE_URL}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.licenses="MIT" \
      farm.query.vgi.transports='["http"]'

ENV PORT=8000 \
    VGI_MEDIA_GIT_COMMIT=${GIT_COMMIT}

WORKDIR /app

# ffmpeg provides the `ffprobe` binary the worker EXECs; curl backs the
# HEALTHCHECK below. Nothing else is needed at runtime.
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# `--chmod` sets the mode in the COPY layer itself, avoiding a second full-size
# layer that a separate `RUN chmod` would create (overlayfs copies on metadata
# change).
COPY --from=build --chmod=0755 /worker /usr/local/bin/vgi-media-worker
COPY --chmod=0755 docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Run unprivileged. The worker is stateless (fixtures/data come from the caller),
# so there is nothing to own or persist.
RUN useradd --create-home --uid 10001 app
USER app

EXPOSE 8000

# Readiness probe for HTTP mode (the Go SDK serves GET /health). Inert for a
# short-lived stdio container, which has no HTTP server.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -fsS "http://localhost:${PORT:-8000}/health" || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["http"]
