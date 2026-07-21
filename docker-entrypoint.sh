#!/bin/sh
# Copyright 2026 Query Farm LLC - https://query.farm
#
# Dispatch the single vgi-media image into one of its transports:
#   http   (default) the HTTP server on $PORT (8000), bound 0.0.0.0 so a
#                    published host port reaches it. The Go SDK serves /health.
#   stdio            a worker DuckDB spawns over stdio (on-host execution).
#   unix <path>      the AF_UNIX launcher transport on a socket path (advanced).
# Any other first argument is exec'd verbatim (escape hatch for debugging).
#
# The worker is stateless — it EXECs ffprobe on whatever file paths / BLOBs the
# caller passes — so there is no /data to create; each mode just exec's the
# binary. HTTP/unix bind 0.0.0.0 on a FIXED port so `-p $PORT:$PORT` and the
# HEALTHCHECK reach the server.
set -e

case "${1:-http}" in
  http)
    shift 2>/dev/null || true
    # Most Go workers hardcode 127.0.0.1:0 (loopback + ephemeral) in --http; the
    # added --http-addr flag lets us bind 0.0.0.0 on a FIXED port in a container.
    exec vgi-media-worker --http --http-addr "0.0.0.0:${PORT:-8000}" "$@"
    ;;
  stdio)
    shift 2>/dev/null || true
    exec vgi-media-worker "$@"
    ;;
  unix)
    shift 2>/dev/null || true
    exec vgi-media-worker --unix "$@"
    ;;
  *)
    exec "$@"
    ;;
esac
