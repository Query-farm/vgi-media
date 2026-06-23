<p align="center">
  <img src="https://raw.githubusercontent.com/Query-farm/vgi/main/docs/vgi-logo.png" alt="Vector Gateway Interface (VGI)" width="320">
</p>

<p align="center"><em>A <a href="https://query.farm">Query.Farm</a> VGI worker for DuckDB.</em></p>

# Video, Audio & Container Metadata via ffprobe in DuckDB

> **vgi-media** · a [Query.Farm](https://query.farm) VGI worker · powered by ffprobe

[![CI](https://github.com/Query-farm/vgi-media/actions/workflows/ci.yml/badge.svg)](https://github.com/Query-farm/vgi-media/actions/workflows/ci.yml)

A [VGI](https://query.farm) worker, written in **Go**, that extracts
**video / audio / container metadata** from media files inside DuckDB/SQL. It
shells out to [`ffprobe`](https://ffmpeg.org/ffprobe.html) (part of **ffmpeg**),
parses its JSON output, and exposes the result as DuckDB scalar and table
functions. Built on the [`vgi-go`](https://github.com/Query-farm/vgi-go) SDK
over stdio. Catalog name: `media`.

```sql
INSTALL vgi FROM community; LOAD vgi;

-- LOCATION is the path to the compiled worker binary.
ATTACH 'media' AS media (TYPE vgi, LOCATION '/path/to/vgi-media-worker');

-- Scalars take either a file PATH (VARCHAR) or media BYTES (BLOB).
SELECT media.media_format('/clips/intro.mp4');   -- 'mov,mp4,m4a,3gp,3g2,mj2'
SELECT media.resolution('/clips/intro.mp4');     -- '1920x1080'
SELECT media.duration('/clips/intro.mp4');       -- 12.34 (seconds)
SELECT media.video_codec('/clips/intro.mp4');    -- 'h264'

-- Probe a BLOB read into DuckDB:
SELECT media.media_format(content)
FROM read_blob('/clips/*.mp4');

-- Table functions: one row per stream / per metadata tag.
SELECT * FROM media.media_streams('/clips/intro.mp4');
SELECT * FROM media.media_tags('/clips/intro.mp4');
```

## Native dependency: ffprobe / ffmpeg

This worker is a thin wrapper around the **`ffprobe`** command-line tool. It is
a **heavyweight native dependency** (like libpostal for `vgi-libpostal`):

- **`ffprobe` must be installed separately and be on `PATH`.** Install ffmpeg:
  - macOS: `brew install ffmpeg`
  - Debian/Ubuntu: `sudo apt-get install ffmpeg`
- Verify with `ffprobe -version`.
- If `ffprobe` is **absent**, the worker returns a **clear, actionable DuckDB
  error** (not a NULL) — a missing native dependency is a configuration problem,
  not "this input is not media".

The worker only **EXECs** the `ffprobe` binary in a subprocess (`ffprobe -v
error -print_format json -show_format -show_streams …`). It does **not** link
any ffmpeg library — see [Licensing](#licensing).

## Input: path or bytes

Every function accepts a single input argument that may be **either**:

| Input type | How it is probed |
| --- | --- |
| **VARCHAR** | Treated as a filesystem path. ffprobe opens the file directly — the most capable mode (it can seek, so end-of-file `moov`-atom MP4s probe fully). |
| **BLOB** | The bytes are piped to `ffprobe … -i pipe:0` via stdin. Non-seekable, so a few container layouts that ffprobe can only parse by seeking may probe less completely than from a path. |

## Functions

### Scalars

| Function | Returns | Description |
| --- | --- | --- |
| `media_format(input)` | `VARCHAR` | Container format name (`format_name`). |
| `duration(input)` | `DOUBLE` | Duration in seconds. |
| `bitrate(input)` | `BIGINT` | Container bit rate (bits/sec). |
| `media_size(input)` | `BIGINT` | Container size in bytes. |
| `stream_count(input)` | `INTEGER` | Number of elementary streams. |
| `video_codec(input)` | `VARCHAR` | Codec of the first video stream. |
| `width(input)` | `INTEGER` | Width of the first video stream (px). |
| `height(input)` | `INTEGER` | Height of the first video stream (px). |
| `resolution(input)` | `VARCHAR` | First video stream as `'WIDTHxHEIGHT'` (e.g. `'1920x1080'`). |
| `fps(input)` | `DOUBLE` | Frames/sec of the first video stream (from `avg_frame_rate`). |
| `audio_codec(input)` | `VARCHAR` | Codec of the first audio stream. |

### Table functions

| Function | Returns | Description |
| --- | --- | --- |
| `media_streams(input)` | `idx INT, type VARCHAR, codec VARCHAR, width INT, height INT, bit_rate BIGINT, duration DOUBLE, channels INT, sample_rate INT` | One row per elementary stream (video/audio/subtitle/data). Columns irrelevant to a stream's type are NULL. |
| `media_tags(input)` | `key VARCHAR, value VARCHAR` | One row per format-level metadata tag (title, artist, encoder, …), ordered by key. |

## Behavior & robustness

- **NULL input → NULL / no rows.** A NULL argument yields a NULL scalar or zero
  table rows, never an error.
- **Non-media / garbage / truncated input → NULL / no rows.** ffprobe rejects
  it; the worker swallows that error. Untrusted media bytes can never crash the
  worker.
- **Bounded work.** Every `ffprobe` invocation runs in a subprocess with a
  timeout and capped `-probesize` / `-analyzeduration`, so adversarial input
  cannot hang or run unbounded.
- **Missing `ffprobe` → clear error.** The single case that surfaces as a DuckDB
  error rather than NULL — it is an actionable configuration problem.

## Build

Requires Go 1.25+ and `ffprobe` on `PATH`.

```sh
make build        # builds ./vgi-media-worker
```

The `vgi-media-worker` binary speaks the VGI protocol over stdio; point a DuckDB
`ATTACH ... (TYPE vgi, LOCATION '…')` at it.

## Test

```sh
make fixtures     # (re)generate the tiny fixtures with ffmpeg (already committed)
make test-unit    # pure-Go unit tests against the committed fixtures
make test-sql     # haybarn-unittest SQL end-to-end against the fixtures
make test         # both
```

`make test-sql` needs [`haybarn-unittest`](https://query.farm) on `PATH`:

```sh
uv tool install haybarn-unittest
export PATH="$HOME/.local/bin:$PATH"
```

### Fixtures

Two tiny, deterministic fixtures are committed under `test/sql/data/` and
generated by `make fixtures`:

```sh
# ~1s of silence, mono, pcm_s16le (audio-only WAV container)
ffmpeg -f lavfi -i anullsrc=r=8000:cl=mono -t 1 -c:a pcm_s16le test/sql/data/silent.wav

# 1s synthetic 320x240 @ 10fps h264 video (video-only MP4 container)
ffmpeg -f lavfi -i testsrc=duration=1:size=320x240:rate=10 \
       -pix_fmt yuv420p -c:v libx264 -movflags +faststart test/sql/data/tiny.mp4
```

## Licensing

- **This worker is licensed MIT** — see [`LICENSE`](./LICENSE). Its only Go
  dependencies are the Go standard library, the Apache Arrow Go library
  (Apache-2.0), and the [`vgi-go`](https://github.com/Query-farm/vgi-go) /
  `vgi-rpc-go` SDK (see those repos for their terms).
- **ffmpeg / ffprobe is a SEPARATE native dependency**, not bundled or linked
  here. Depending on how it was built, ffmpeg is **LGPL-2.1+** or **GPL-2.0+**.
  The worker only **executes** the `ffprobe` binary as a subprocess — it does
  **not** link against any ffmpeg library — so there is **no license
  entanglement** between this MIT-licensed worker and ffmpeg's (L)GPL terms.
  Users supply their own `ffprobe` install and are responsible for complying
  with their ffmpeg build's license.

---

## Authorship & License

Written by [Query.Farm](https://query.farm).

Copyright 2026 Query Farm LLC - https://query.farm

