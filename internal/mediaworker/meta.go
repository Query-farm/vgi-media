// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Shared helpers for the per-object discovery/description metadata that the
// vgi-lint strict profile expects on EVERY function and table.
//
// Each function/table surfaces these in its FunctionMetadata.Tags:
//   - vgi.title (VGI124)      — human-friendly display name
//   - vgi.doc_llm (VGI112)    — Markdown narrative description aimed at LLMs
//   - vgi.doc_md (VGI113)     — Markdown narrative description for human docs
//   - vgi.keywords (VGI126)   — JSON array of search terms/synonyms
//
// vgi.source_url is set ONLY on the catalog object (see main.go's CatalogInfo);
// per-object source_url is redundant and is rejected by VGI139.

// jsonMarshal marshals v to a compact JSON string WITHOUT HTML escaping, so
// operators like '>' inside example SQL survive as-is rather than becoming
// '>'. Returns an error only when v is not JSON-encodable.
func jsonMarshal(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// keywordsJSON converts a comma-separated keyword string into a JSON array of
// trimmed, non-empty strings, e.g. "a, b" -> ["a","b"]. VGI138 requires
// vgi.keywords to be a JSON array of strings, not a comma-separated string.
func keywordsJSON(csv string) string {
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// exampleSampleBaseURL is where the catalog's EXAMPLE queries read their media:
// the committed fixtures (test/sql/data/tiny.mp4, silent.wav) served from
// GitHub at the v0.4.0 release tag, so the URL resolves now and forever.
//
// vgi-lint executes every example, and users copy them, from anywhere — not
// from this repo. A local path (a repo fixture, or a placeholder like
// '/clips/intro.mp4') only works inside the repo: elsewhere the worker swallows
// the probe failure and every scalar returns NULL and every table function zero
// rows (VGI913/VGI516). ffprobe opens https:// URLs itself, so the same string
// argument works for the scalars AND the table functions (which only accept a
// constant argument, so they cannot take read_blob() bytes).
const exampleSampleBaseURL = "https://raw.githubusercontent.com/Query-farm/vgi-media/v0.4.0/test/sql/data/"

// exampleVideoURL is a 1 s, 320x240, 10 fps H.264 MP4 (video-only).
const exampleVideoURL = exampleSampleBaseURL + "tiny.mp4"

// exampleAudioURL is a 1 s mono pcm_s16le WAV (audio-only). Audio examples
// (audio_codec) must probe it: tiny.mp4 has no audio stream.
const exampleAudioURL = exampleSampleBaseURL + "silent.wav"

// exampleVideoPath / exampleAudioPath are the absolute paths to the committed
// fixtures used by the agent test task PROMPTS (AgentTestTasksJSON), resolved
// once at package init. The tasks' grader reference_sql lives in the repo-root
// sidecar vgi-agent-tests.yaml and names the same fixtures repo-relatively, so
// the agent suite runs from the repo (as vgi-lint does in CI). Examples use the
// URLs above instead. If the fixtures cannot be located (e.g. the binary was
// copied elsewhere), we fall back to placeholder paths.
var exampleVideoPath = resolveFixture("tiny.mp4", "/clips/intro.mp4")

// exampleAudioPath is the audio-only counterpart to exampleVideoPath (the
// committed silent.wav fixture).
var exampleAudioPath = resolveFixture("silent.wav", "/clips/theme.wav")

// resolveFixture returns the absolute path to a committed fixture named `name`
// under test/sql/data, searching upward from the current working directory and
// from the running executable's directory. Returns `fallback` if not found.
func resolveFixture(name, fallback string) string {
	rel := filepath.Join("test", "sql", "data", name)
	var roots []string
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots, cwd)
	}
	if exe, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(exe))
	}
	for _, root := range roots {
		dir := root
		for i := 0; i < 8; i++ {
			candidate := filepath.Join(dir, rel)
			if abs, err := filepath.Abs(candidate); err == nil {
				if _, statErr := os.Stat(abs); statErr == nil {
					return abs
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return fallback
}

// SchemaExampleQueriesJSON builds the schema-level vgi.example_queries carrier
// (VGI506/VGI515) as a described-JSON list. Every example's SQL reads the
// committed video fixture by its tag-pinned URL (exampleVideoURL) — the same
// file the executable examples use — so each query returns real values and rows
// wherever the worker runs. Each example carries a non-empty description (VGI515) and does real
// projection/filtering rather than a bare SELECT * (VGI514).
func SchemaExampleQueriesJSON() string {
	v := sqlEscape(exampleVideoURL)
	type described struct {
		Description string `json:"description"`
		SQL         string `json:"sql"`
	}
	examples := []described{
		{
			Description: "Summarize a media file in one row: container format, duration in seconds, and elementary-stream count.",
			SQL:         "SELECT media.main.media_format('" + v + "') AS format, media.main.duration('" + v + "') AS seconds, media.main.stream_count('" + v + "') AS streams",
		},
		{
			Description: "Read the first video stream's codec, resolution, and frame rate.",
			SQL:         "SELECT media.main.video_codec('" + v + "') AS codec, media.main.resolution('" + v + "') AS resolution, media.main.fps('" + v + "') AS fps",
		},
		{
			Description: "List every elementary stream in a media file, one row per stream, ordered by stream index.",
			SQL:         "SELECT idx, type, codec FROM media.main.media_streams('" + v + "') ORDER BY idx",
		},
		{
			Description: "List the container-level metadata tags of a media file as key/value rows, ordered by tag name.",
			SQL:         "SELECT key, value FROM media.main.media_tags('" + v + "') ORDER BY key",
		},
	}
	// Composed examples (composed.go): queries combining several of the
	// catalog's objects.
	for _, c := range composedExamples() {
		examples = append(examples, described{Description: c.Description, SQL: c.SQL})
	}
	b, err := jsonMarshal(examples)
	if err != nil {
		return "[]"
	}
	return b
}

// objectTags builds the standard per-object discovery/description tags.
//
// relativePath is retained for call-site documentation of the implementing file
// (internal/mediaworker), but is no longer emitted as a per-object
// vgi.source_url tag — VGI139 keeps source_url only on the catalog object.
func objectTags(title, descriptionLLM, descriptionMD, keywords, relativePath, category string) map[string]string {
	_ = relativePath
	return map[string]string{
		"vgi.title":    title,
		"vgi.doc_llm":  descriptionLLM,
		"vgi.doc_md":   descriptionMD,
		"vgi.keywords": keywordsJSON(keywords),
		// VGI409/411: name a category defined in the schema's vgi.categories
		// registry (see cmd/vgi-media-worker/main.go).
		"vgi.category": category,
	}
}

// AgentTestTasksJSON builds the catalog's vgi.agent_test_tasks suite (VGI152)
// as a JSON string. The tasks are constructed at runtime so each task's prompt
// references the committed video fixture's resolved ABSOLUTE path
// (exampleVideoPath / resolveFixture) — the same file the executable examples
// use. Only name + prompt are published (VGI416): each task's grader
// reference_sql lives in the private repo-root sidecar vgi-agent-tests.yaml,
// keyed by task name, which vgi-lint discovers on its own. The sidecar names the
// fixtures by their repo-relative path (test/sql/data/...), which the worker
// resolves against its working directory — the repo root, where vgi-lint runs
// in CI. Keep the sidecar in sync when a task changes. Each prompt names the
// exact output column(s) it wants, because simulate grades strictly on column
// names and values.
func AgentTestTasksJSON() string {
	p := exampleVideoPath
	a := exampleAudioPath
	dir := filepath.Dir(exampleVideoPath)
	type task struct {
		Name   string `json:"name"`
		Prompt string `json:"prompt"`
	}
	tasks := []task{
		// --- container-level scalars ---
		{
			Name:   "container-format",
			Prompt: "For the media file at path '" + p + "', return its container format name in a single column named format.",
		},
		{
			Name:   "duration-seconds",
			Prompt: "Return the playback duration in seconds of the media file at path '" + p + "' as a single column named duration_seconds.",
		},
		{
			Name:   "stream-count",
			Prompt: "How many elementary streams does the media file at path '" + p + "' contain? Return the count as a single column named stream_count.",
		},
		{
			// bitrate varies with the encoder, so grade a stable threshold
			// predicate rather than an exact bits-per-second value.
			Name:   "has-bitrate",
			Prompt: "Does the media file at path '" + p + "' report a positive overall container bit rate (in bits per second)? Return a single boolean column named has_bitrate.",
		},
		{
			Name:   "media-size-bytes",
			Prompt: "Return the size in bytes of the media file at path '" + p + "' in a single column named size_bytes.",
		},
		// --- video scalars ---
		{
			Name:   "video-codec",
			Prompt: "Return the codec name of the first video stream of the media file at path '" + p + "' in a single column named codec.",
		},
		{
			Name:   "video-width",
			Prompt: "Return the pixel width of the first video stream of the media file at path '" + p + "' in a single column named width.",
		},
		{
			Name:   "video-height",
			Prompt: "Return the pixel height of the first video stream of the media file at path '" + p + "' in a single column named height.",
		},
		{
			Name:   "video-resolution",
			Prompt: "Return the resolution of the first video stream of the media file at path '" + p + "', formatted as WIDTHxHEIGHT, in a single column named resolution.",
		},
		{
			// fps is a floating-point rate; grade a stable threshold predicate
			// rather than the raw double to avoid rounding/rename flakiness.
			Name:   "fps-above-five",
			Prompt: "Does the first video stream of the media file at path '" + p + "' play at more than 5 frames per second? Return a single boolean column named above_5fps.",
		},
		// --- audio scalar (probes the audio-only fixture) ---
		{
			Name:   "audio-codec",
			Prompt: "Return the codec name of the first audio stream of the media file at path '" + a + "' in a single column named codec.",
		},
		// --- table functions ---
		{
			Name:   "list-streams",
			Prompt: "List every elementary stream in the media file at path '" + p + "'. Return one row per stream with columns idx, type, and codec, ordered by idx ascending.",
		},
		{
			// The tag values (e.g. the encoder string) vary by ffmpeg version,
			// so grade a stable existence predicate over the media_tags rows.
			Name:   "has-encoder-tag",
			Prompt: "Does the media file at path '" + p + "' carry a container-level metadata tag named 'encoder'? Return a single boolean column named has_encoder.",
		},
		// --- composed: combine several of the catalog's objects ---
		{
			// media_format + duration over every file in the fixture folder.
			Name: "format-duration-rollup",
			Prompt: "For every media file in the folder '" + dir + "', group the files by container format. " +
				"Return columns format, n (the number of files with that format) and total_seconds (their summed " +
				"duration in seconds, rounded to 1 decimal place), one row per format, ordered by format.",
		},
		{
			// media_streams joined to media_tags.
			Name: "video-codec-with-brand",
			Prompt: "For the media file at path '" + p + "', return one row per video stream with two columns: codec " +
				"(the stream's codec name) and major_brand (the value of the file's container-level 'major_brand' " +
				"metadata tag), ordered by stream index.",
		},
		// --- discovery view ---
		{
			Name:   "registry-video-functions",
			Prompt: "Using the browsable registry of media functions in this catalog, list the names of every function in the 'video' category, ordered alphabetically, in a single column named name.",
		},
	}
	b, err := json.Marshal(tasks)
	if err != nil {
		return "[]"
	}
	return string(b)
}
