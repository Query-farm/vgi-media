// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
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

// exampleVideoPath / exampleAudioPath are the absolute paths to the committed
// fixtures used by the EXECUTABLE example queries, resolved once at package init
// so the linter's example-execution pass (VGI901/902/906) runs against real
// media and the table-function examples return rows.
//
// The strict linter EXECUTES every example. A placeholder path like
// '/clips/intro.mp4' executes cleanly (the worker swallows probe failures to
// NULL / no rows) but the table functions then return ZERO rows, tripping
// VGI902 ("example returned no rows"). Pointing the examples at the committed
// fixtures (test/sql/data/tiny.mp4, silent.wav) makes them return real rows
// wherever the worker is run from the repo. If the fixtures cannot be located
// (e.g. the binary was copied elsewhere), we fall back to the placeholder paths,
// which still execute cleanly.
var exampleVideoPath = resolveFixture("tiny.mp4", "/clips/intro.mp4")

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
// as a JSON string. The tasks are constructed at runtime so both each task's
// prompt and the grader's reference_sql reference the committed video fixture's
// resolved ABSOLUTE path (exampleVideoPath / resolveFixture) — the same file the
// executable examples use — so `vgi-lint simulate` grades against real rows
// wherever the worker is launched from the repo. Each prompt names the exact
// output column(s) it wants, because simulate grades strictly on column names
// and values.
func AgentTestTasksJSON() string {
	p := exampleVideoPath
	esc := sqlEscape(p)
	type task struct {
		Name         string `json:"name"`
		Prompt       string `json:"prompt"`
		ReferenceSQL string `json:"reference_sql"`
	}
	tasks := []task{
		{
			Name:         "container-format",
			Prompt:       "For the media file at path '" + p + "', return its container format name in a single column named format.",
			ReferenceSQL: "SELECT media.main.media_format('" + esc + "') AS format;",
		},
		{
			Name:         "duration-seconds",
			Prompt:       "Return the playback duration in seconds of the media file at path '" + p + "' as a single column named duration_seconds.",
			ReferenceSQL: "SELECT media.main.duration('" + esc + "') AS duration_seconds;",
		},
		{
			Name:         "stream-count",
			Prompt:       "How many elementary streams does the media file at path '" + p + "' contain? Return the count as a single column named stream_count.",
			ReferenceSQL: "SELECT media.main.stream_count('" + esc + "') AS stream_count;",
		},
		{
			Name:         "video-resolution",
			Prompt:       "Return the resolution of the first video stream of the media file at path '" + p + "', formatted as WIDTHxHEIGHT, in a single column named resolution.",
			ReferenceSQL: "SELECT media.main.resolution('" + esc + "') AS resolution;",
		},
		{
			Name:         "list-streams",
			Prompt:       "List every elementary stream in the media file at path '" + p + "'. Return one row per stream with columns idx, type, and codec, ordered by idx ascending.",
			ReferenceSQL: "SELECT idx, type, codec FROM media.main.media_streams('" + esc + "') ORDER BY idx;",
		},
	}
	b, err := json.Marshal(tasks)
	if err != nil {
		return "[]"
	}
	return string(b)
}
