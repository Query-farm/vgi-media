# CLAUDE.md — vgi-media

Contributor/agent notes. User-facing docs live in `README.md`; this is the
"how it's built and where the sharp edges are" companion. Modeled directly on
the `vgi-grpc` Go worker (the proven Go template).

## What this is

A [VGI](https://query.farm) worker (Go) that extracts **video / audio /
container metadata** via **`ffprobe`** (from ffmpeg) and exposes it as DuckDB
scalar + table functions. Built on the [`vgi-go`](https://github.com/Query-farm/vgi-go)
SDK over stdio. Catalog name: `media`.

## Layout

```
cmd/vgi-media-worker/main.go   stdio entry point; assembles the worker + catalog
internal/mediaworker/
  ffprobe.go                   subprocess seam: ProbePath/ProbeBytes → ProbeResult; accessors
  input.go                     probeRow(): VARCHAR-path vs BLOB-stdin dispatch; CatalogName
  scalars.go                   the 11 scalar functions (string/int32/int64/float64 shapes)
  tables.go                    media_streams + media_tags table fns; Register(w); helpers
  *_test.go                    Go tests against the committed fixtures
test/sql/*.test                haybarn-unittest sqllogictest — authoritative E2E
test/sql/data/                 committed tiny fixtures (silent.wav, tiny.mp4)
Makefile                       build / fixtures / test-unit / test-sql / lint
```

To add a function: add an accessor to `ffprobe.go` (parsing the relevant
ffprobe field), then a scalar in `scalars.go` (`registerScalars`) or a table fn
in `tables.go`, and register it.

## ffprobe subprocess (the heart of it)

The worker is a thin wrapper around the **`ffprobe`** CLI — a **heavyweight
native dependency** that must be installed separately and on `PATH` (Homebrew:
`brew install ffmpeg`). `runFFprobe` in `ffprobe.go` is the single subprocess
seam:

```
ffprobe -v error -hide_banner -analyzeduration 10M -probesize 16M \
        -print_format json -show_format -show_streams <input>
```

- **VARCHAR input → file path**, passed as the positional `<input>` so ffprobe
  opens (and can seek within) the file.
- **BLOB input → `-i pipe:0`** with the bytes written to the child's stdin.
  Non-seekable; a few seek-only container layouts probe less completely. This
  path-vs-stdin split lives in `input.go::probeRow`.
- Bounded by a `context.WithTimeout(probeTimeout)` **and** capped
  `-probesize`/`-analyzeduration` so untrusted/truncated/garbage bytes can never
  hang or run unbounded.

### NULL vs swallow vs error (important)

`probeRow` / `swallow` encode the robustness contract:

- **NULL input or unsupported column type** → `(nil, nil)` → SQL NULL / no rows.
- **ffprobe ran but rejected the input** (non-media, truncated, …) → the error
  is **swallowed** to `(nil, nil)` → NULL / no rows. Untrusted bytes never crash
  a query.
- **`ffprobe` binary missing** → `ErrFFprobeNotFound` is **propagated** to
  DuckDB as a clear, actionable error. This is the one case that is NOT a NULL.
  Detected via `exec.ErrNotFound` (PATH lookup) **and** `fs.ErrNotExist` (an
  absolute path that doesn't exist) — the latter is what the
  `TestProbeMissingBinary` override hits.

ffprobe emits every numeric value as a JSON **string** (`"1.000000"`, `"30/1"`),
so the accessors (`Duration()`, `FPS()`, `BitRateInt()`, …) parse lazily and
return `(value, ok)`; `ok=false` maps to NULL. `parseRate` handles ffmpeg
fractions including `"0/0"` (unknown → not ok).

## The Go SDK worker pattern (reusable; same as vgi-grpc)

`main()` assembles a `*vgi.Worker`, registers functions, `RunStdio()`.

**Scalars** are `vgi.TypedScalarFunc[A]` wrapped with `vgi.AsScalarFunction[A]`.
The single argument struct uses a **column arg** so the input accepts BOTH a
VARCHAR path and a BLOB:

```go
type inputArg struct {
    Input arrow.Array `vgi:"pos=0,const=false,doc=..."`  // const=false → arrow_type="any"
}
```

`const=false` makes it a column arg (read from the input batch), and declaring
it as `arrow.Array` advertises `arrow_type="any"` so DuckDB passes VARCHAR or
BLOB unchanged. The Process body reads `batch.Column(0)` directly and builds a
**nullable** output (AppendNull on probe failure) — that is why we hand-roll the
builder loop instead of `vgi.MapColumn` (which only propagates *input* nulls).

**Table functions** are `vgi.TypedTableFunc[S]` wrapped with
`vgi.AsTableFunction[S]`. They have no streamed input batch, so the input is a
**const scalar arg** with a `type=any` tag override (note: the tag key is
`type`, NOT `arrow_type`):

```go
type tableArgs struct {
    Input string `vgi:"pos=0,type=any,doc=..."`
}
```

The body does NOT call `vgi.BindArgs` for the input; it reads the raw column via
`params.Args.GetColumn(0)` (see `probeArg0`) so a BLOB column isn't forced
through a string binder.

## Sharp edges (learned the hard way)

1. **Table-function state is `gob`-encoded by the SDK** between `NewState` and
   `Process` (the SDK now panics at registration if the state isn't
   gob-encodable). So state `S` must have **exported, gob-safe fields only** —
   no `arrow.Record`, no interfaces, no unexported fields. Pattern used here:
   fetch + flatten the ffprobe result into plain Go slices in `NewState`
   (`Rows []streamRow`, `Tags []tagKV`), carry optional fields as explicit
   `Has* bool` companions, plus `Done bool`, and **rebuild the Arrow batch in
   `Process`**.

2. **`haybarn-unittest` silently SKIPS `require vgi`.** Use an explicit
   `statement ok` / `LOAD vgi;` instead — every `.test` here does. Tests use
   `# group: [vgi_media]`, `require-env VGI_MEDIA_WORKER` /
   `require-env VGI_MEDIA_DATA_DIR`, and
   `ATTACH 'media' AS media (TYPE vgi, LOCATION '${VGI_MEDIA_WORKER}')`.

3. **The arg tag key is `type`, not `arrow_type`.** `type=any` advertises the
   polymorphic "any" type for the table functions' const input.

4. **Nullable output needs a hand-rolled builder.** `vgi.MapColumn` only nulls
   the output where the *input* is null; here the output is null when *ffprobe
   fails or the field is absent*, which is independent of input nullness — so
   each scalar appends explicitly (`b.Append` / `b.AppendNull`).

5. **`fs.ErrNotExist` matters for the missing-binary path.** An absolute
   non-existent ffprobe path fails at `cmd.Run()` with a `*fs.PathError`, not
   `*exec.Error`; both are mapped to `ErrFFprobeNotFound`.

## Fixtures (how `make test-sql` works)

Two tiny deterministic fixtures are **committed** under `test/sql/data/` and
regenerated by `make fixtures` (commands documented in the README and the
Makefile):

- `silent.wav` — ~1s mono silence, pcm_s16le (audio-only).
- `tiny.mp4` — 1s 320x240 @ 10fps h264 (video-only; carries an `encoder` tag).

`make test-sql` builds the worker, exports `VGI_MEDIA_WORKER` (the binary path,
used as the ATTACH `LOCATION`) and `VGI_MEDIA_DATA_DIR` (read by the `.test`
files to build fixture paths), then runs `haybarn-unittest --test-dir .
"test/sql/*"`. No mock server is needed — the fixtures are the test corpus.

## Test inventory

- **Go (`make test-unit`)** — `internal/mediaworker/ffprobe_test.go` (path +
  bytes probing of both fixtures, garbage-bytes survival, missing-binary error,
  `parseRate`) and `functions_test.go` (`media_streams`/`media_tags` `NewState`
  over the fixtures, NULL→no-rows, garbage-path→no-rows, garbage-BLOB column
  through `probeRow`).
- **SQL (`make test-sql`)** — `test/sql/media_probe.test` (scalar metadata:
  format, ROUND(duration), resolution, codecs, fps, stream_count, NULL→NULL,
  garbage-BLOB→NULL) and `test/sql/media_tables.test` (`media_streams` rows
  ORDER BY idx, audio NULL video cols, `media_tags`, NULL/garbage→no rows).

## Conventions

- Source files start with `// Copyright 2026 Query Farm LLC - https://query.farm`.
- `gofmt`, `go vet`, and `go test ./...` must be clean before committing.
- The worker only **EXECs** ffprobe; it never links an ffmpeg library — so the
  MIT worker stays clear of ffmpeg's (L)GPL terms.
