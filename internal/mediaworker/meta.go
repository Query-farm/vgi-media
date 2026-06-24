// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"os"
	"path/filepath"
)

// Shared helpers for the per-object discovery/description metadata that the
// vgi-lint strict profile expects on EVERY function and table.
//
// Each function/table surfaces these in its FunctionMetadata.Tags:
//   - vgi.title (VGI124)      — human-friendly display name
//   - vgi.doc_llm (VGI112)    — Markdown narrative description aimed at LLMs
//   - vgi.doc_md (VGI113)     — Markdown narrative description for human docs
//   - vgi.keywords (VGI126)   — comma-separated search terms/synonyms
//   - vgi.source_url (VGI128) — link to the implementing source file
//
// sourceURL(file) builds the canonical GitHub blob URL for a source file so
// every object points at exactly where it is implemented.

// sourceBase is the base GitHub blob URL for source files in this repo (pinned
// to main).
const sourceBase = "https://github.com/Query-farm/vgi-media/blob/main/internal/mediaworker"

// sourceURL builds the implementation vgi.source_url for a file under
// internal/mediaworker, e.g. sourceURL("scalars.go").
func sourceURL(relativePath string) string {
	return sourceBase + "/" + relativePath
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

// objectTags builds the five standard per-object discovery/description tags.
//
// relativePath is the implementing file relative to internal/mediaworker.
func objectTags(title, descriptionLLM, descriptionMD, keywords, relativePath string) map[string]string {
	return map[string]string{
		"vgi.title":      title,
		"vgi.doc_llm":    descriptionLLM,
		"vgi.doc_md":     descriptionMD,
		"vgi.keywords":   keywords,
		"vgi.source_url": sourceURL(relativePath),
	}
}
