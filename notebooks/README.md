# Media metadata notebooks

[01 · A two-second container does not promise two seconds of video](01_container_length_is_not_video_length.ipynb) compares file and stream durations, applies a local review threshold, and measures which metadata survives non-seekable BLOB input. All inputs are explicitly synthetic; no media is downloaded or played.

## Install and run from this directory

Requires Python 3.13–3.14, [uv](https://docs.astral.sh/uv/), internet for initial packages/release/extension downloads, and **ffprobe from ffmpeg on PATH**. On macOS use `brew install ffmpeg`; Debian/Ubuntu use `sudo apt-get install ffmpeg`. Check `ffprobe -version`. ffmpeg itself is needed only to regenerate the committed fixtures.

```sh
cd notebooks
uv sync --locked
uv run --locked python execute.py
uv run --locked python -m ipykernel install --user --name vgi-media-notebooks --display-name 'Python (vgi-media notebooks)'
```

Select that kernel in your notebook application, then Restart Kernel → Run All. The runner uses a fresh temporary kernel without requiring global kernel installation. It sets the working directory to this directory, writes outputs atomically only after success, and preserves the previous successful default on failure. Copying this entire directory is sufficient; no worker checkout or local build is used.

The visible literal `github-auto://Query-farm/vgi-media@v0.4.0` downloads a platform release and verifies its SHA-256. Release tag resolves to `557bb5e77fb3deae63d836960d94d64b26b0d8c5`. Notebook Haybarn is locked to `2.0.0rc11`. The separately installed ffprobe is **not** pinned by uv; the notebook reports its version. Validation used ffprobe 9.0.2 on macOS arm64. Upgrading the worker tag or ffprobe requires rerunning both examples and reviewing availability changes.

## One-parameter experiment

```sh
uv run --locked python execute.py --parameters '{"DURATION_GAP_SECONDS":1.1}' --output-dir validation/gap-1.1
```

Default is 0.1 seconds. Only the strict absolute-duration-difference threshold changes. The runner refuses overrides without a separate output directory and rejects unknown parameter names. Experiment copies include fixtures, branding, this README, standard, environment and runner. `validation/`, extracted charts, local environments and runtime artifacts are ignored; the main notebook retains its successful embedded outputs.

## Inputs and interpretation

`fixtures/tiny.mp4` and `silent.wav` are unchanged copies of this repository's committed `test/sql/data/` synthetic fixtures. `audio-tail.mp4` contains a one-second video test pattern and two-second sine-wave audio. `not-media.txt` is deliberately invalid media. The SQL NULL is a separate absent input. `fixtures/manifest.json` records file hashes, generator version and the exact audio-tail generation command. Setup verifies every hash before probing. Do not replace files while keeping an old manifest.

The same native worker performs every path and BLOB probe. Calls are materialized before charts; each scalar still launches its own ffprobe process. Stream table functions use explicit constant paths because they cannot take lateral path columns. The corpus is four files; there is no recursive directory scan or remote URL.

A duration difference compares reported lengths, not exact start/end alignment: stream start timestamps are absent from this schema. It cannot establish synchronization, missing frames, corruption or playback quality. The threshold is illustrative policy. NULL, zero and inapplicable fields stay distinct. Input errors can be swallowed to NULL/no rows; missing ffprobe propagates as a configuration error.

**Release-specific limitation:** v0.4.0 does not fill a missing BLOB `media_size` from the input byte count; newer source does. The notebook shows the release output separately from `octet_length(content)`. WAV duration can also be unavailable from the pipe even when the same bytes yield a duration via a seekable path. These results depend on the format, release and native ffprobe version; they are not universal promises about all media.

[Notebook standard](STANDARD.md) · [Query.Farm](https://query.farm/) · [VGI](https://query.farm/vgi/)
