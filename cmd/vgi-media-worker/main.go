// Copyright 2026 Query Farm LLC - https://query.farm

// Command vgi-media-worker is a VGI worker that extracts video / audio /
// container metadata from media files via ffprobe (part of ffmpeg) and exposes
// it as DuckDB SQL scalar and table functions. It speaks the VGI protocol over
// stdio.
//
// ffprobe is a heavyweight NATIVE dependency: it must be installed separately
// (e.g. `brew install ffmpeg`) and available on PATH. The worker only EXECs the
// ffprobe binary in a subprocess; it does not link any ffmpeg library.
package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/Query-farm/vgi-go/vgi"
	"github.com/Query-farm/vgi-media/internal/mediaworker"
)

func main() {
	// Accept --http for HTTP transport and --unix for the AF_UNIX launcher
	// transport; default is stdio. Unknown launcher flags are tolerated (the
	// VGI extension varies argv to key its worker cache), so we filter to flags
	// we actually define before parsing.
	httpMode := flag.Bool("http", false, "Run as an HTTP server instead of stdio")
	httpAddr := flag.String("http-addr", "127.0.0.1:0", "HTTP listen address (ignored unless --http); default binds an ephemeral loopback port for dev/CI")
	unixPath := flag.String("unix", "", "Serve the AF_UNIX launcher transport on this socket path instead of stdio")
	logFlags := vgi.RegisterLoggingFlags(flag.CommandLine)
	_ = flag.CommandLine.Parse(filterKnownFlags(os.Args[1:], map[string]bool{
		"log-level":  true,
		"log-format": true,
		"log-logger": true,
		"http-addr":  true,
		"unix":       true,
	}))
	if err := logFlags.Apply(); err != nil {
		log.Fatalf("logging flags: %v", err)
	}

	sourceURL := "https://github.com/Query-farm/vgi-media"
	w := vgi.NewWorker(
		vgi.WithCatalogName(mediaworker.CatalogName),
		vgi.WithCatalogComment("Extract video/audio/container metadata from media files via ffprobe."),
		vgi.WithCatalogInfo(vgi.CatalogInfo{
			Name:      mediaworker.CatalogName,
			SourceURL: &sourceURL,
		}),
		vgi.WithCatalogTags(map[string]string{
			"source":    "vgi-media",
			"vgi.title": "Media Metadata Extraction",
			"vgi.keywords": `["media","video","audio","ffprobe","ffmpeg","metadata","codec",` +
				`"duration","bitrate","resolution","fps","streams","container format",` +
				`"mp4","mkv","wav","transcoding"]`,
			"vgi.doc_llm": "Extract video, audio, and container metadata from media files " +
				"with ffprobe (ffmpeg). Scalars take a file path (`VARCHAR`) or media bytes (`BLOB`) and " +
				"return container format, duration, bit rate, size, stream count, and per-stream video " +
				"(codec, width, height, resolution, fps) and audio (codec) attributes. Table functions " +
				"list every elementary stream (media_streams) and every format-level metadata tag " +
				"(media_tags). Use for media inventory, transcoding triage, and quality/conformance checks in SQL.",
			"vgi.doc_md": "# Media Metadata Extraction in SQL with ffprobe\n\n" +
				"![FFmpeg logo](https://ffmpeg.org/ffmpeg-logo.png)\n\n" +
				"Inspect video, audio, and container files directly from DuckDB SQL: read codec, " +
				"resolution, duration, bitrate, frame rate, and embedded metadata tags from MP4, MKV, " +
				"MOV, WAV, MP3, WebM, and every other format that [FFmpeg](https://ffmpeg.org) can " +
				"open — no manual transcoding, no external scripts.\n\n" +
				"This extension is for data engineers, media pipelines, and anyone who needs to take " +
				"inventory of a media library, triage transcoding jobs, or run quality and conformance " +
				"checks at scale. Instead of shelling out to a command-line tool and parsing JSON by " +
				"hand, you query your media the same way you query any other table, and join the results " +
				"against the rest of your warehouse.\n\n" +
				"## How it works\n\n" +
				"Under the hood the worker runs [ffprobe](https://ffmpeg.org/ffprobe.html), the media " +
				"analyzer that ships with the [FFmpeg](https://github.com/FFmpeg/FFmpeg) project, in a " +
				"sandboxed subprocess with bounded probe size and timeouts so that truncated or " +
				"untrusted bytes can never hang a query. Every function accepts either a filesystem " +
				"path (`VARCHAR`) or the raw media bytes (`BLOB`), so you can probe files on disk or `BLOB` " +
				"columns already loaded into DuckDB. ffprobe's findings are surfaced over Apache Arrow " +
				"as native SQL columns; missing or unparseable fields return NULL rather than erroring.\n\n" +
				"## What you can read\n\n" +
				"The worker groups its surface into a few kinds of reading:\n\n" +
				"- **Container-level facts** — one value per file, such as the format name, playback " +
				"duration, overall bit rate, byte size, and how many elementary streams the file holds.\n" +
				"- **Video attributes** — for the first video track: its codec, pixel width and height, " +
				"a combined resolution string, and the average frame rate.\n" +
				"- **Audio attributes** — for the first audio track, such as its codec.\n" +
				"- **Full enumeration** — table functions that return one row per elementary stream, " +
				"and one row per format-level metadata tag, when you need every detail rather than a " +
				"single summary value.\n\n" +
				"## When to use it\n\n" +
				"Reach for this worker whenever media files are part of your data and you want their " +
				"technical metadata available in SQL: cataloguing an asset library, validating that " +
				"uploads meet a resolution or codec policy, or driving transcoding decisions from a " +
				"query rather than a bespoke script.\n\n" +
				"See the official [ffprobe documentation](https://ffmpeg.org/ffprobe.html) and the " +
				"broader [FFmpeg documentation](https://ffmpeg.org/documentation.html) for details on " +
				"the underlying analyzer.",
			// Fixed analyst-task suite (VGI152/VGI920). Built at runtime so both
			// the prompts and the grader's reference_sql point at the committed
			// fixture's absolute path wherever the worker is launched from.
			"vgi.agent_test_tasks":   mediaworker.AgentTestTasksJSON(),
			"vgi.author":             "Query.Farm",
			"vgi.copyright":          "Copyright 2026 Query Farm LLC - https://query.farm",
			"vgi.license":            "MIT",
			"vgi.support_contact":    "https://github.com/Query-farm/vgi-media/issues",
			"vgi.support_policy_url": "https://github.com/Query-farm/vgi-media/blob/main/README.md",
		}),
		vgi.WithSchemaComments(map[string]string{
			"main": "Media metadata extraction functions (scalars + table functions) over ffprobe.",
		}),
		vgi.WithSchemaTags(map[string]map[string]string{
			"main": {
				"vgi.title": "Media — main",
				"vgi.keywords": `["media","video","audio","ffprobe","metadata","codec",` +
					`"duration","bitrate","resolution","fps","media_streams","media_tags",` +
					`"container format"]`,
				// VGI123 classifying tags use BARE keys (not vgi.-namespaced) so the
				// schema is findable by facet/topic.
				"domain":   "media",
				"category": "metadata-extraction",
				"topic":    "video-audio-inspection",
				"vgi.doc_llm": "Media metadata functions: container-level scalars (format, " +
					"duration, bitrate, size, stream_count), per-stream video/audio scalars (codec, " +
					"width, height, resolution, fps), and table functions for elementary streams " +
					"(media_streams) and format-level metadata tags (media_tags).",
				"vgi.doc_md": "## Media metadata functions\n\n" +
					"Read technical metadata from video, audio, and container files over Apache Arrow, " +
					"backed by ffprobe. Every function accepts either a media file path (`VARCHAR`) or " +
					"the raw media bytes (`BLOB`), and missing or unparseable fields return NULL rather " +
					"than erroring.\n\n" +
					"The surface is organised into a few groups:\n\n" +
					"- **Container** — one value per file: format, duration, bit rate, size, and " +
					"stream count.\n" +
					"- **Video** — the first video track's codec, width, height, resolution, and " +
					"frame rate.\n" +
					"- **Audio** — the first audio track's codec.\n" +
					"- **Enumeration** — table functions that return one row per elementary stream and " +
					"one row per format-level metadata tag.\n\n" +
					"Use this schema for media inventory, transcoding triage, and quality or " +
					"conformance checks in SQL.",
				// Navigation/SEO category registry (VGI413) — each function/table
				// below is filed into one of these via a vgi.category tag.
				"vgi.categories": `[` +
					`{"name":"container","description":"Container-level facts about a media file: format name, duration, bit rate, byte size, and elementary-stream count."},` +
					`{"name":"video","description":"Attributes of the first video stream: codec, pixel width and height, resolution string, and average frame rate."},` +
					`{"name":"audio","description":"Attributes of the first audio stream, such as its codec name."},` +
					`{"name":"enumeration","description":"Table functions that enumerate every elementary stream and every format-level metadata tag in a file."},` +
					`{"name":"discovery","description":"Browsable registry view of the catalog's media functions, so an agent can discover every callable object and its result type without first probing a file."}` +
					`]`,
				// VGI506/VGI515 representative, described example queries for the
				// schema. Built at runtime so each query points at the committed
				// fixture and executes cleanly with real rows.
				"vgi.example_queries": mediaworker.SchemaExampleQueriesJSON(),
			},
		}),
	)
	mediaworker.Register(w)

	if *httpMode {
		if err := w.RunHttp(*httpAddr); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *unixPath != "" {
		// AF_UNIX launcher transport: serve on the given socket path. The SDK
		// prints "UNIX:<path>" once listening; idleTimeout=0 disables the
		// self-shutdown timer (the launcher/CI owns the process lifecycle).
		if err := w.RunUnix(*unixPath, 0); err != nil {
			log.Fatal(err)
		}
		return
	}
	w.RunStdio()
}

// filterKnownFlags drops argv tokens for flags this binary doesn't define, so
// launcher-injected differentiation flags don't abort flag parsing. Flags named
// in valueFlags consume the following token as their value.
func filterKnownFlags(args []string, valueFlags map[string]bool) []string {
	defined := map[string]bool{}
	flag.CommandLine.VisitAll(func(f *flag.Flag) { defined[f.Name] = true })
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		hasInlineValue := strings.ContainsRune(name, '=')
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if !defined[name] {
			continue
		}
		out = append(out, a)
		if valueFlags[name] && !hasInlineValue && i+1 < len(args) {
			i++
			out = append(out, args[i])
		}
	}
	return out
}
