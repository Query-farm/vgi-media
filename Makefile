# vgi-media Makefile
#
# A VGI worker (Go) that extracts video/audio/container metadata via ffprobe
# and exposes it as DuckDB scalar + table functions. Targets:
#
#   make build       Build the worker binary
#   make fixtures    (Re)generate the tiny test fixtures with ffmpeg
#   make test-unit   Run the pure-Go unit tests (against committed fixtures)
#   make test-sql    Run the haybarn-unittest SQL E2E against the fixtures
#   make test        test-unit + test-sql
#   make fmt         gofmt -w
#   make vet         go vet
#   make lint        golangci-lint (if installed) else vet
#   make clean       Remove built binaries
#
# NATIVE DEPENDENCY: ffprobe (from ffmpeg) must be on PATH for the worker to
# run, and ffmpeg for `make fixtures`. Install via Homebrew: `brew install ffmpeg`.
#
# test-sql needs haybarn-unittest on PATH:
#   uv tool install haybarn-unittest
#   export PATH="$$HOME/.local/bin:$$PATH"

WORKER_BIN  := vgi-media-worker
WORKER_CMD  := ./cmd/vgi-media-worker

TEST_DIR     := .
TEST_PATTERN := test/sql/*
DATA_DIR     := test/sql/data

# Absolute path to the built worker (the VGI extension launches it via LOCATION).
WORKER_PATH := $(CURDIR)/$(WORKER_BIN)

.PHONY: build fixtures test test-unit test-sql fmt vet lint clean

build:
	go build -o $(WORKER_BIN) $(WORKER_CMD)

# Deterministic tiny fixtures. Committed under test/sql/data/, so this only
# needs re-running if you intentionally change the fixtures.
#   - silent.wav : ~1s of silence, mono, pcm_s16le (audio-only container)
#   - tiny.mp4   : 1s synthetic 320x240 @ 10fps h264 video (video-only container)
fixtures:
	ffmpeg -hide_banner -loglevel error -y \
		-f lavfi -i anullsrc=r=8000:cl=mono -t 1 -c:a pcm_s16le $(DATA_DIR)/silent.wav
	ffmpeg -hide_banner -loglevel error -y \
		-f lavfi -i testsrc=duration=1:size=320x240:rate=10 \
		-pix_fmt yuv420p -c:v libx264 -movflags +faststart $(DATA_DIR)/tiny.mp4

test: test-unit test-sql

test-unit:
	go test ./...

# Build the worker, then run the haybarn SQL suite. The .test files read the
# fixture paths from VGI_MEDIA_DATA_DIR and ATTACH the worker via
# VGI_MEDIA_WORKER (its absolute path).
test-sql: build
	VGI_MEDIA_WORKER="$(WORKER_PATH)" \
	VGI_MEDIA_DATA_DIR="$(CURDIR)/$(DATA_DIR)" \
		haybarn-unittest --test-dir "$(TEST_DIR)" "$(TEST_PATTERN)"

fmt:
	gofmt -w .

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found; running go vet instead"; \
		go vet ./...; \
	fi

clean:
	rm -f $(WORKER_BIN)
