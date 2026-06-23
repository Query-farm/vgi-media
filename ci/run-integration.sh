#!/usr/bin/env bash
# Copyright 2026 Query Farm LLC - https://query.farm
#
# Run this repo's sqllogictest suite (test/sql/*.test) against the vgi-media
# VGI worker, using a prebuilt standalone `haybarn-unittest` and the signed
# community `vgi` extension — no C++ build from source. See ci/README.md.
#
# The media worker shells out to `ffprobe` (from ffmpeg) to read committed
# fixture files. The .test files reference those fixtures by absolute path via
# VGI_MEDIA_DATA_DIR (mirroring `make test-sql`); ffprobe must be on PATH.
#
# Required environment:
#   HAYBARN_UNITTEST   path to the haybarn-unittest binary
#   VGI_MEDIA_WORKER   worker LOCATION the .test files ATTACH (the built Go
#                      worker binary the vgi extension spawns over stdio)
# Optional:
#   VGI_MEDIA_DATA_DIR absolute fixtures dir (default: <repo>/test/sql/data)
#   STAGE              scratch dir for the preprocessed test tree (default: mktemp)
set -euo pipefail

: "${HAYBARN_UNITTEST:?path to the haybarn-unittest binary}"
: "${VGI_MEDIA_WORKER:?worker LOCATION (the built Go worker binary)}"

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/.." && pwd)"
STAGE="${STAGE:-$(mktemp -d)}"

# The committed fixtures (tiny.mp4, silent.wav) are referenced by the .test
# files via absolute ${VGI_MEDIA_DATA_DIR}/<file> paths, so point at the repo's
# data dir directly — no need to copy them into the stage.
export VGI_MEDIA_DATA_DIR="${VGI_MEDIA_DATA_DIR:-$REPO/test/sql/data}"
echo "Using fixtures from $VGI_MEDIA_DATA_DIR"

# --- Stage the preprocessed tests -------------------------------------------
echo "Staging preprocessed tests into $STAGE ..."
mkdir -p "$STAGE/test/sql"
for f in "$REPO"/test/sql/*.test; do
  awk -f "$HERE/preprocess-require.awk" "$f" > "$STAGE/test/sql/$(basename "$f")"
done

cd "$STAGE"

# Warm the extension cache once: vgi from the signed community channel. A miss
# here is only a warning — the per-test LOAD vgi; (the .test files load it
# explicitly) is what actually gates each file, and it needs vgi already
# INSTALLed into the runner's extension dir.
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

# Run the whole suite in one invocation, streaming the runner's native
# sqllogictest report. Any failed assertion exits non-zero and fails the job.
echo "Running suite (worker: $VGI_MEDIA_WORKER) ..."
"$HAYBARN_UNITTEST" "test/sql/*"
