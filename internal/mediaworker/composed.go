// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"path/filepath"

	"github.com/Query-farm/vgi-go/vgi"
)

// Composed examples: realistic queries that combine two or more of this
// catalog's objects in one statement, rather than demonstrating one function in
// isolation. Each is defined ONCE here and referenced from the schema-level
// vgi.example_queries list (SchemaExampleQueriesJSON) and from the example list
// of the object that drives the query (so the vgi-lint agent simulator, which
// only shows per-object examples, sees it too).
//
// The table functions take a constant argument (DuckDB rejects LATERAL column
// parameters for them), so a per-file table-function composition is written
// against one literal path; the multi-file composition uses the scalars over
// DuckDB's glob().
//
// Built from the committed fixtures' resolved absolute paths, like every other
// example, so they execute and return rows wherever the worker runs from the
// repo.

// streamDriftExample compares each stream's own duration with the container
// duration (media_streams + duration) — a common sanity check for remuxed or
// truncated files whose tracks end before/after the container says.
func streamDriftExample() vgi.CatalogExample {
	v := sqlEscape(exampleVideoPath)
	return vgi.CatalogExample{
		Description: "Compare every stream's duration with the container's duration and show the drift in seconds " +
			"(media_streams + duration); a non-zero drift flags a track that ends early or late, as in a truncated or badly remuxed file.",
		SQL: "SELECT s.idx, s.type, s.codec, s.duration AS stream_seconds, " +
			"media.main.duration('" + v + "') AS container_seconds, " +
			"round(abs(s.duration - media.main.duration('" + v + "')), 3) AS drift_seconds " +
			"FROM media.main.media_streams('" + v + "') AS s ORDER BY s.idx",
	}
}

// fileCardExample builds a one-row summary of a file from four objects: the
// container format and duration scalars, the stream layout aggregated from
// media_streams, and the major_brand tag from media_tags.
func fileCardExample() vgi.CatalogExample {
	v := sqlEscape(exampleVideoPath)
	return vgi.CatalogExample{
		Description: "One-row file card: container format and duration, the stream layout aggregated from media_streams " +
			"(e.g. 'video:h264'), and the MP4 major_brand container tag from media_tags.",
		SQL: "SELECT media.main.media_format('" + v + "') AS format, " +
			"round(media.main.duration('" + v + "'), 2) AS seconds, " +
			"(SELECT string_agg(s.type || ':' || s.codec, ', ' ORDER BY s.idx) FROM media.main.media_streams('" + v + "') AS s) AS stream_layout, " +
			"(SELECT t.value FROM media.main.media_tags('" + v + "') AS t WHERE t.key = 'major_brand') AS major_brand",
	}
}

// folderInventoryExample probes every file in a folder with several scalars
// and classifies each as video or audio-only from which codec is present.
func folderInventoryExample() vgi.CatalogExample {
	dir := sqlEscape(filepath.Join(filepath.Dir(exampleVideoPath), "*"))
	return vgi.CatalogExample{
		Description: "Inventory a folder of media files: for each file, its container format, whether it is video or audio-only " +
			"(from video_codec / audio_codec), the codec, the video resolution (NULL for audio-only), and the duration.",
		SQL: "SELECT parse_filename(file) AS file_name, media.main.media_format(file) AS format, " +
			"CASE WHEN media.main.video_codec(file) IS NOT NULL THEN 'video' " +
			"WHEN media.main.audio_codec(file) IS NOT NULL THEN 'audio-only' ELSE 'unknown' END AS kind, " +
			"coalesce(media.main.video_codec(file), media.main.audio_codec(file)) AS codec, " +
			"media.main.resolution(file) AS resolution, round(media.main.duration(file), 2) AS seconds " +
			"FROM glob('" + dir + "') ORDER BY file_name",
	}
}

// composedExamples is the schema-level list of every composed example.
func composedExamples() []vgi.CatalogExample {
	return []vgi.CatalogExample{folderInventoryExample(), fileCardExample(), streamDriftExample()}
}
