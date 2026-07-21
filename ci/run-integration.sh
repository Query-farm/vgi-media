#!/usr/bin/env bash
# Copyright 2026 Query Farm LLC - https://query.farm
#
# Run this repo's sqllogictest suite (test/sql/*.test) against the vgi-media
# VGI worker, using a prebuilt standalone `haybarn-unittest` and the signed
# community `vgi` extension — no C++ build from source. See ci/README.md.
#
# The media worker shells out to `ffprobe` (from ffmpeg) to read committed
# fixture files. The .test files reference those fixtures by ABSOLUTE path via
# VGI_MEDIA_DATA_DIR (mirroring `make test-sql`); ffprobe must be on PATH. No
# mock server is needed — the committed fixtures are the corpus.
#
# Multi-transport: the same suite runs over whichever transport the TRANSPORT
# env var selects, by changing what `VGI_MEDIA_WORKER` resolves to (the vgi
# extension picks the transport from the ATTACH LOCATION string):
#
#   subprocess (default)  VGI_MEDIA_WORKER = the stdio worker binary
#                         -> extension spawns it over stdin/stdout.
#   http                  start `<worker> --http` (prints "PORT:<n>"), parse the
#                         port, VGI_MEDIA_WORKER = http://127.0.0.1:<port>.
#   unix                  start `<worker> --unix /tmp/media.sock` (prints
#                         "UNIX:<path>"), VGI_MEDIA_WORKER = unix:///tmp/media.sock.
#
# For http/unix the worker runs out-of-band (not spawned by DuckDB); because the
# fixtures are referenced by ABSOLUTE path, the out-of-band worker resolves them
# regardless of its cwd, so ffprobe opens the same files in every transport.
#
# Required environment:
#   HAYBARN_UNITTEST   path to the haybarn-unittest binary
#   VGI_MEDIA_WORKER   for TRANSPORT=subprocess: the worker LOCATION the .test
#                      files ATTACH (the built Go worker binary, spawned over
#                      stdio). For http/unix this is OVERRIDDEN by this script,
#                      but the binary it points at is reused to launch the
#                      out-of-band server, so it must still be the worker path.
# Optional:
#   TRANSPORT          subprocess (default) | http | unix
#   VGI_MEDIA_DATA_DIR absolute fixtures dir (default: <repo>/test/sql/data)
#   STAGE              scratch dir for the preprocessed test tree (default: mktemp)
set -euo pipefail

: "${HAYBARN_UNITTEST:?path to the haybarn-unittest binary}"
: "${VGI_MEDIA_WORKER:?worker LOCATION (the built Go worker binary)}"

TRANSPORT="${TRANSPORT:-subprocess}"
case "$TRANSPORT" in
  subprocess|http|unix) ;;
  *) echo "ERROR: unknown TRANSPORT='$TRANSPORT' (expected subprocess|http|unix)" >&2; exit 2 ;;
esac

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"
STAGE="${STAGE:-$(mktemp -d)}"

# The committed fixtures (tiny.mp4, silent.wav) are referenced by the .test
# files via absolute ${VGI_MEDIA_DATA_DIR}/<file> paths, so point at the repo's
# data dir directly — no need to copy them into the stage.
export VGI_MEDIA_DATA_DIR="${VGI_MEDIA_DATA_DIR:-$REPO/test/sql/data}"
echo "Using fixtures from $VGI_MEDIA_DATA_DIR"

# The worker binary the subprocess transport ATTACHes to is also the binary we
# launch out-of-band for http/unix. Capture it before we possibly overwrite
# VGI_MEDIA_WORKER with a URL.
WORKER_BIN="$VGI_MEDIA_WORKER"

WORKER_PID=""
UNIX_SOCK=""
cleanup() {
  # Preserve the script's exit status (this runs on EXIT).
  local rc=$?
  if [ -n "$WORKER_PID" ]; then kill "$WORKER_PID" 2>/dev/null || true; wait "$WORKER_PID" 2>/dev/null || true; fi
  if [ -n "$UNIX_SOCK" ]; then rm -f "$UNIX_SOCK"; fi
  return "$rc"
}
trap cleanup EXIT

# --- Per-transport: resolve VGI_MEDIA_WORKER (the ATTACH LOCATION) -----------
case "$TRANSPORT" in
  subprocess)
    echo "Transport: subprocess/stdio — VGI_MEDIA_WORKER=$VGI_MEDIA_WORKER"
    ;;

  http)
    # Pre-launched HTTP hook: if VGI_MEDIA_WORKER is already an http(s):// URL
    # (e.g. a warm Docker container the image_test started), use it verbatim
    # rather than spawning a local binary. Defaults (a binary path) are
    # unchanged: fall through to spawning `<worker> --http` as before.
    case "$WORKER_BIN" in
      http://*|https://*)
        echo "Transport: http — using pre-launched worker at $WORKER_BIN"
        export VGI_MEDIA_WORKER="$WORKER_BIN"
        ;;
      *)
    WORKER_PORT_FILE="$(mktemp)"
    echo "Transport: http — starting '$WORKER_BIN --http' ..."
    "$WORKER_BIN" --http >"$WORKER_PORT_FILE" 2>/dev/null &
    WORKER_PID=$!
    WPORT=""
    for _ in $(seq 1 50); do
      WPORT="$(sed -n 's/^PORT:\([0-9][0-9]*\)$/\1/p' "$WORKER_PORT_FILE" 2>/dev/null | head -1)"
      [ -n "$WPORT" ] && break
      kill -0 "$WORKER_PID" 2>/dev/null || { echo "ERROR: http worker exited before reporting a port" >&2; cat "$WORKER_PORT_FILE" >&2 || true; exit 1; }
      sleep 0.2
    done
    rm -f "$WORKER_PORT_FILE"
    if [ -z "$WPORT" ]; then
      echo "ERROR: http worker did not report a port" >&2
      exit 1
    fi
    # Bare scheme://host:port with NO path (the extension POSTs each RPC method
    # at <LOCATION>/<method>, mounted at the server root).
    export VGI_MEDIA_WORKER="http://127.0.0.1:$WPORT"
    echo "HTTP worker listening on $VGI_MEDIA_WORKER (pid $WORKER_PID)"
        ;;
    esac
    ;;

  unix)
    UNIX_SOCK="${TMPDIR:-/tmp}/media.$$.sock"
    rm -f "$UNIX_SOCK"
    WORKER_OUT_FILE="$(mktemp)"
    echo "Transport: unix — starting '$WORKER_BIN --unix $UNIX_SOCK' ..."
    "$WORKER_BIN" --unix "$UNIX_SOCK" >"$WORKER_OUT_FILE" 2>/dev/null &
    WORKER_PID=$!
    READY=""
    for _ in $(seq 1 50); do
      if grep -q '^UNIX:' "$WORKER_OUT_FILE" 2>/dev/null && [ -S "$UNIX_SOCK" ]; then
        READY=1; break
      fi
      kill -0 "$WORKER_PID" 2>/dev/null || { echo "ERROR: unix worker exited before the socket was ready" >&2; cat "$WORKER_OUT_FILE" >&2 || true; exit 1; }
      sleep 0.2
    done
    rm -f "$WORKER_OUT_FILE"
    if [ -z "$READY" ]; then
      echo "ERROR: unix worker did not report a ready socket at $UNIX_SOCK" >&2
      exit 1
    fi
    export VGI_MEDIA_WORKER="unix://$UNIX_SOCK"
    echo "Unix worker listening on $VGI_MEDIA_WORKER (pid $WORKER_PID)"
    ;;
esac

# --- Stage the preprocessed tests -------------------------------------------
# TEST_PATTERN (default: every test/sql/*.test) selects which source .test files
# to stage/run — a repo-relative glob so the image_test can smoke a single file.
# The default is unchanged (all tests).
TEST_PATTERN="${TEST_PATTERN:-test/sql/*.test}"
echo "Staging preprocessed tests into $STAGE (pattern: $TEST_PATTERN) ..."
mkdir -p "$STAGE/test/sql"
staged=0
for f in "$REPO"/$TEST_PATTERN; do
  [ -f "$f" ] || continue
  awk -f "$HERE/preprocess-require.awk" "$f" > "$STAGE/test/sql/$(basename "$f")"
  staged=$((staged + 1))
done
if [ "$staged" -eq 0 ]; then
  echo "ERROR: TEST_PATTERN '$TEST_PATTERN' matched no .test files under $REPO" >&2
  exit 1
fi

# The HTTP transport drives the worker-RPC POSTs through DuckDB's HTTP client,
# only registered when `httpfs` is loaded. The .test files only `LOAD vgi`, so
# over HTTP those POSTs fail with an "HTTP"-flavoured error (which the runner
# silently SKIPS). Inject a signed httpfs INSTALL+LOAD after each `LOAD vgi;`
# for the http transport only.
if [ "$TRANSPORT" = "http" ]; then
  echo "Transport http: injecting 'LOAD httpfs' (required for the worker HTTP RPC) ..."
  for f in "$STAGE"/test/sql/*.test; do
    awk '
      { print }
      /^LOAD[ \t]+vgi;[ \t]*$/ {
        print "";
        print "statement ok";
        print "INSTALL httpfs FROM core;";
        print "";
        print "statement ok";
        print "LOAD httpfs;";
      }
    ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
  done
fi

cd "$STAGE"

# Warm the extension cache once: vgi from the signed community channel.
echo "Warming the extension cache (vgi from community) ..."
mkdir -p "$STAGE/test"
cat > "$STAGE/test/_warm.test" <<'EOF'
# name: test/_warm.test
# group: [warm]
statement ok
INSTALL vgi FROM community;
EOF
"$HAYBARN_UNITTEST" "test/_warm.test" >/dev/null 2>&1 || echo "::warning::extension warm step did not fully succeed"
rm -f "$STAGE/test/_warm.test"

# Run the whole suite in one invocation, capturing the runner's native
# sqllogictest report so we can both stream it AND guard against a silent skip.
#
# IMPORTANT: the runner SKIPS (exit 0) a test whose error message matches a
# built-in network-error allowlist that includes "HTTP". A broken HTTP transport
# would otherwise show "All tests were skipped" and go GREEN having run nothing.
# We detect that and fail explicitly.
echo "Running suite (transport: $TRANSPORT, worker: $VGI_MEDIA_WORKER) ..."
RUN_LOG="$STAGE/run.log"
set +e
"$HAYBARN_UNITTEST" "test/sql/*" 2>&1 | tee "$RUN_LOG"
RUN_RC="${PIPESTATUS[0]}"
set -e

if [ "$RUN_RC" -ne 0 ]; then
  echo "ERROR: suite failed (transport: $TRANSPORT, rc=$RUN_RC)" >&2
  exit "$RUN_RC"
fi

if grep -q 'All tests were skipped' "$RUN_LOG"; then
  echo "ERROR: every test was SKIPPED on transport '$TRANSPORT' (the runner's" >&2
  echo "       built-in network-error skip swallowed the real error). This is" >&2
  echo "       NOT a pass. Skip reason reported by the runner:" >&2
  grep -A3 'Skipped tests for the following reasons' "$RUN_LOG" >&2 || true
  exit 1
fi
