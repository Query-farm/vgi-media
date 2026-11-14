# Media Metadata example

[01 · A two-second container does not promise two seconds of video](01_container_length_is_not_video_length.ipynb)
retains the original duration/pipe story and adds first-stream file cards, container tags and
reference-driven field availability. All fourteen named public SQL surfaces are used in this
one inventory workflow. The input corpus is synthetic: no media is downloaded, played or captured.

Read saved outputs on GitHub or use **Open in Colab**, then Run All. A single downloaded notebook
retrieves hash-verified fixtures through the shared package. Actual hosted Colab is untested;
the separate preloaded Python 3.12 notebook-only check exercises the public Python/fixture path.

## Run from this directory

Requires Python 3.11+, [uv](https://docs.astral.sh/uv/), internet for first-time package/release/extension
and fixture downloads, and **ffprobe from ffmpeg on PATH**. On macOS use `brew install ffmpeg`;
on Debian/Ubuntu use `sudo apt-get install ffmpeg`. The visible notebook setup can install ffmpeg
with apt-get if ffprobe is missing in a root Linux runtime, as typically used by Colab. That
Linux installation branch is not exercised by the macOS validation. `ffmpeg` itself is needed
only to regenerate fixtures; metadata calls execute `ffprobe`.

```sh
uv sync --locked --extra runner
uv run --locked --extra runner vgi-notebooks
```

The released [shared package v0.3.22](https://github.com/Query-farm/vgi-example-notebooks/tree/v0.3.22) supplies
canonical branding, SVG formatting, isolated SQL setup and an atomic runner. Colab installs
only the base package, preserving compatible preloaded data/plotting libraries and its kernel.
The optional `[runner]` extra and local `uv.lock` preserve the publication environment.
The complete notebooks directory runs independently of the worker checkout.

```sql
INSTALL vgi FROM community;
LOAD vgi;
ATTACH 'media' (TYPE vgi, LOCATION 'github-auto://Query-farm/vgi-media@v0.4.0');
```

This visible literal launch downloads the native platform release and verifies its SHA-256.
The tag resolves to `557bb5e77fb3deae63d836960d94d64b26b0d8c5`. No local Go build is needed.
Haybarn 2.0.0rc11 is pinned separately. `ffprobe` is an external native prerequisite, **not**
pinned by uv; its actual version is reported in saved outputs. Validation uses ffprobe 9.0.2
on macOS arm64. Native package versions and format support can differ on Linux/Colab.

`connect(globals())` uses empty temporary SQL-secret storage, disables persistent secrets and
prevents JupySQL from opening saved default connections. This local computation worker needs
no credentials. Cleanup closes the in-memory database and removes the temporary secret store.

Optional local kernel:

```sh
uv run --locked --extra runner python -m ipykernel install --user \
  --name vgi-media-notebooks --display-name 'Python (vgi-media notebooks)'
```

Restart and Run All after editing. Parameters, dependency setup, ATTACH and analytical SQL remain
visible; Python presentation helpers carry folding metadata. Defaults embed native SVG with
committed image links for GitHub; independent experiment copies retain native SVG only.
Logs, charts exported for review, environments and experiment copies are ignored.

## One-parameter experiment

```sh
uv run --locked --extra runner vgi-notebooks \
  --parameters '{"DURATION_GAP_SECONDS":1.1}' --output-dir validation/gap-1.1
```

The default 0.1-second threshold flags the one-second absolute container/video length difference.
At 1.1 seconds it is no longer flagged. The comparison is strict `>`; only policy changes.
Media bytes, measured durations, codecs, dimensions, rates, tags, registry annotations and pipe
availability remain unchanged. This does not repair media or establish playback quality.
Restore 0.1 for publication. Finite numeric values 0–2 are accepted, excluding booleans.

Overrides require a separate destination. Unknown parameter names, source-overwriting destinations
and invalid values fail without replacing the last successful default/experiment. The runner
keeps complete portable companions with each experiment copy.

## Inputs and provenance

`fixtures/tiny.mp4` and `silent.wav` are unchanged copies of the repository's committed synthetic
`test/sql/data/` fixtures. `audio-tail.mp4` combines a one-second video test pattern and two-second
sine-wave audio. `not-media.txt` is deliberately invalid media, while a separate SQL NULL tests
absence. [manifest.json](fixtures/manifest.json) records SHA-256 hashes, generator version and the
exact audio-tail generation command. Setup checks every hash. Public companions are pinned to
`268008d1165158b7ccfacc9900b7be01dd0dd08d`. Do not replace inputs without updating provenance.

The same released worker performs path and BLOB probes. Each scalar launches its own ffprobe;
materialization avoids repeating those calls during display. Stream/tag table functions use
explicit constant paths because their arguments cannot be lateral path columns. The corpus is
four files; there is no directory recursion, remote URL, camera or user-media access.

## Interpretation and version limits

A reported duration difference compares lengths, not exact start/end alignment. Stream start
timestamps are absent from this schema. It cannot establish synchronization, missing frames,
corruption or whether a player holds the final frame. The threshold is an illustrative review
policy. NULL, zero and inapplicable fields remain distinct; rejected inputs can become NULL/no
rows, while a missing ffprobe executable propagates a configuration error.

The video scalar fields refer to the first video stream, and audio codec to the first audio
stream. They are file-card projections, not a complete track listing. Both synthetic video files
report 320×240 and 10 average frames/s; this is not a measured frame count or proof of equal content.
Container bitrate is bits/s; size is bytes. `8 × bytes / seconds` is a whole-file rate estimate,
not a video-only rate or a quality score. Container encoder/brand tags are editable metadata,
not authenticated provenance or proof of successful decoding.

**Release-specific difference:** v0.4.0 does not fill a missing BLOB `media_size` from the input
byte count; current source adds that fallback. The example shows the released NULL separately
from `octet_length(content)` and does not repair worker output. WAV duration can also be absent
through the non-seekable pipe while the same bytes yield a duration through a seekable path.
These observations depend on the format, release and ffprobe version, not every media file.

The `media_functions` reference view supplies category/result-type labels for observed duration
and size availability. Its thirteen rows describe the thirteen functions; the view is itself a
fourteenth named SQL surface. A catalog type does not fill a missing measurement.

## Coverage and validation

[coverage.json](coverage.json) maps **14/14** named surfaces to visible executed SQL, returned
results and narrative interpretation. Complete runtime functions/tables/views exports under
[coverage/](coverage/) establish thirteen functions, zero physical tables and one native view.
Current public source exposes the same interface; the BLOB-size semantic difference is recorded
separately. Evidence counts fourteen exact released-runtime demonstrations and zero exact current
runtime executions. Current-interface compatibility review does not imply equal BLOB-size values.

Default and detached experiment executions check twelve file-card/reference invariants plus the
original stream/count/byte/NULL/policy relationships. Fixture hashes and unchanged first-stage
results are compared; the path-versus-pipe chart stays identical. A fresh preloaded notebook-only
setup checks the public fixture download path. Figures and actual GitHub SVG rendering are reviewed.

Existing green [source CI 43575197037](https://github.com/Query-farm/vgi-media/actions/runs/43575197037)
is reused for unchanged worker code; no source build, full suite or source/container CI dispatch
is performed for this notebook-only migration. Original source, Go manifests and unrelated edits
are preserved. Actual hosted Colab and its apt-get installation branch remain untested.
