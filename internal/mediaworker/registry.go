// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import "github.com/Query-farm/vgi-go/vgi"

// media_functions is a browsable, credential-free registry VIEW that lists every
// callable object this catalog exposes — one row per scalar and table function —
// with its category, kind, SQL return type, and a one-line description.
//
// WHY A VIEW (VGI146): every other object in this catalog is a *function* that
// needs a media path/BLOB argument before it returns anything, so an agent has
// nothing to browse to discover the surface. This view is backed entirely by an
// inline VALUES list, so `SELECT * FROM media.main.media_functions` scans with no
// arguments and no ffprobe call — the agent can read the whole function catalog,
// pick the one it needs, and learn its return type before ever probing a file.
// Kept in lockstep with registerScalars (scalars.go) and Register (tables.go).

// registryDefinition is the SQL backing the media_functions view: a static
// VALUES table of (name, kind, category, result_type, description).
const registryDefinition = `SELECT * FROM (VALUES
  ('media_format',  'scalar', 'container',   'VARCHAR', 'Container/wrapper format name of a media file (ffprobe format_name).'),
  ('duration',      'scalar', 'container',   'DOUBLE',  'Total playback duration of a media file, in seconds.'),
  ('bitrate',       'scalar', 'container',   'BIGINT',  'Overall container bit rate of a media file, in bits per second.'),
  ('media_size',    'scalar', 'container',   'BIGINT',  'Size of a media file, in bytes.'),
  ('stream_count',  'scalar', 'container',   'INTEGER', 'Number of elementary streams (video/audio/subtitle/data) in a container.'),
  ('video_codec',   'scalar', 'video',       'VARCHAR', 'Codec name of the first video stream (e.g. h264, hevc, vp9, av1).'),
  ('width',         'scalar', 'video',       'INTEGER', 'Pixel width of the first video stream.'),
  ('height',        'scalar', 'video',       'INTEGER', 'Pixel height of the first video stream.'),
  ('resolution',    'scalar', 'video',       'VARCHAR', 'Resolution of the first video stream formatted as WIDTHxHEIGHT.'),
  ('fps',           'scalar', 'video',       'DOUBLE',  'Average frame rate (frames per second) of the first video stream.'),
  ('audio_codec',   'scalar', 'audio',       'VARCHAR', 'Codec name of the first audio stream (e.g. aac, mp3, opus, flac).'),
  ('media_streams', 'table',  'enumeration', 'TABLE',   'One row per elementary stream, with per-stream codec and dimensions.'),
  ('media_tags',    'table',  'enumeration', 'TABLE',   'One row per container-level metadata tag as a key/value pair.')
) AS t(name, kind, category, result_type, description)`

// registerRegistryView registers the media_functions discovery view (VGI146).
func registerRegistryView(w *vgi.Worker) {
	w.RegisterCatalogView("main", vgi.CatalogView{
		Name:       "media_functions",
		Definition: registryDefinition,
		Comment:    "Browsable registry of every media metadata function in this catalog, with its category, kind, and SQL return type.",
		ColumnComments: map[string]string{
			"name":        "Function name as called in SQL (e.g. media_format, media_streams).",
			"kind":        "Whether the object is a 'scalar' function or a 'table' function.",
			"category":    "Navigation category: 'container', 'video', 'audio', or 'enumeration'.",
			"result_type": "SQL type the scalar returns, or 'TABLE' for a table function.",
			"description": "One-line summary of what the function returns.",
		},
		Tags: map[string]string{
			"vgi.title": "Media Function Registry",
			"vgi.doc_llm": "A browsable registry view listing every scalar and table function this " +
				"catalog exposes — one row per function with its name, kind (scalar/table), navigation " +
				"category (container/video/audio/enumeration), SQL result type, and a one-line " +
				"description. Query it with no arguments to discover the full media-metadata surface " +
				"and each function's return type before probing any file. Filter by category or kind " +
				"to narrow to the functions you need.",
			"vgi.doc_md": "## Media function registry\n\n" +
				"A browsable, argument-free view that catalogues every media metadata function in this " +
				"worker. Because every other object is a function that needs a media path or `BLOB` before " +
				"it returns anything, this view is the entry point for discovery: read it to see the full " +
				"surface and each function's return type, then call the function you need.\n\n" +
				"Columns:\n\n" +
				"- `name` — function name as called in SQL.\n" +
				"- `kind` — `scalar` or `table`.\n" +
				"- `category` — `container`, `video`, `audio`, or `enumeration`.\n" +
				"- `result_type` — the SQL type a scalar returns, or `TABLE` for a table function.\n" +
				"- `description` — a one-line summary.\n\n" +
				"For example, list only the video functions and their return types, or count how many " +
				"functions fall into each category.",
			"vgi.keywords": keywordsJSON("registry, catalog, functions, discovery, browse, list functions, " +
				"function list, media functions, capabilities, index"),
			"vgi.category": "discovery",
			// VGI123 classifying tags use BARE keys (reused schema-wide vocabulary).
			"domain": "media",
			"topic":  "video-audio-inspection",
			"vgi.example_queries": `[` +
				`{"description":"List every video-category function and its return type, alphabetically.",` +
				`"sql":"SELECT name, result_type FROM media.main.media_functions WHERE category = 'video' ORDER BY name"},` +
				`{"description":"Count how many functions fall into each navigation category.",` +
				`"sql":"SELECT category, count(*) AS n FROM media.main.media_functions GROUP BY category ORDER BY category"}` +
				`]`,
		},
	})
}
