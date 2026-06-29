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
	unixPath := flag.String("unix", "", "Serve the AF_UNIX launcher transport on this socket path instead of stdio")
	logFlags := vgi.RegisterLoggingFlags(flag.CommandLine)
	_ = flag.CommandLine.Parse(filterKnownFlags(os.Args[1:], map[string]bool{
		"log-level":  true,
		"log-format": true,
		"log-logger": true,
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
				"with ffprobe (ffmpeg). Scalars take a file path (VARCHAR) or media bytes (BLOB) and " +
				"return container format, duration, bit rate, size, stream count, and per-stream video " +
				"(codec, width, height, resolution, fps) and audio (codec) attributes. Table functions " +
				"list every elementary stream (media_streams) and every format-level metadata tag " +
				"(media_tags). Use for media inventory, transcoding triage, and quality/conformance checks in SQL.",
			"vgi.doc_md": "# Media Metadata Extraction in SQL with ffprobe\n\n" +
				"Inspect video, audio, and container files directly from DuckDB SQL: read codec, " +
				"resolution, duration, bitrate, frame rate, and embedded metadata tags from MP4, MKV, " +
				"MOV, WAV, MP3, WebM, and every other format that [FFmpeg](https://ffmpeg.org) can " +
				"open — no manual transcoding, no external scripts.\n\n" +
				"This extension is for data engineers, media pipelines, and anyone who needs to take " +
				"inventory of a media library, triage transcoding jobs, or run quality and conformance " +
				"checks at scale. Instead of shelling out to a command-line tool and parsing JSON by " +
				"hand, you query your media the same way you query any other table, and join the results " +
				"against the rest of your warehouse.\n\n" +
				"Under the hood the worker runs [`ffprobe`](https://ffmpeg.org/ffprobe.html), the media " +
				"analyzer that ships with the [FFmpeg](https://github.com/FFmpeg/FFmpeg) project, in a " +
				"sandboxed subprocess with bounded probe size and timeouts so that truncated or " +
				"untrusted bytes can never hang a query. Every function accepts either a file path " +
				"(VARCHAR) or the raw media bytes (BLOB), so you can probe files on disk or BLOB columns " +
				"already loaded into DuckDB. ffprobe's findings are surfaced over Apache Arrow as native " +
				"SQL columns; missing or unparseable fields return NULL rather than erroring.\n\n" +
				"Container-level scalars report top-level facts about a file: `media_format`, " +
				"`duration`, `bitrate`, `media_size`, and `stream_count`. Per-stream scalars summarize " +
				"the first video or audio stream: `video_codec`, `width`, `height`, `resolution`, " +
				"`fps`, and `audio_codec`. For full detail, two table functions enumerate everything " +
				"inside a file: `media_streams` returns one row per elementary stream (index, type, " +
				"codec, and dimensions), and `media_tags` returns one row per format-level metadata tag " +
				"(title, encoder, creation time, and more). A typical query looks like " +
				"`SELECT media_format(path), duration(path), resolution(path) FROM files;` or " +
				"`SELECT * FROM media_streams('/clips/intro.mp4');`.\n\n" +
				"See the official [ffprobe documentation](https://ffmpeg.org/ffprobe.html) and the " +
				"broader [FFmpeg documentation](https://ffmpeg.org/documentation.html) for details on " +
				"the underlying analyzer.",
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
				"vgi.doc_md": "Media metadata extraction functions over Apache Arrow, backed by " +
					"ffprobe. Container-level scalars report format, duration, bitrate, size, and " +
					"stream count; per-stream scalars report the first video/audio codec, width, " +
					"height, resolution, and frame rate. Table functions media_streams and media_tags " +
					"enumerate every elementary stream and every format-level metadata tag. Every " +
					"function accepts a media file path or raw media bytes. Use this schema for media " +
					"inventory, transcoding triage, and quality/conformance checks in SQL.",
				// VGI506 representative example queries for the schema.
				"vgi.example_queries": "SELECT media.main.media_format('/clips/intro.mp4');\n" +
					"SELECT media.main.duration('/clips/intro.mp4');\n" +
					"SELECT media.main.video_codec('/clips/intro.mp4'), media.main.resolution('/clips/intro.mp4');\n" +
					"SELECT media.main.stream_count('/clips/intro.mp4');\n" +
					"SELECT idx, type, codec FROM media.main.media_streams('/clips/intro.mp4') ORDER BY idx;\n" +
					"SELECT key, value FROM media.main.media_tags('/clips/intro.mp4') ORDER BY key;",
			},
		}),
	)
	mediaworker.Register(w)

	if *httpMode {
		if err := w.RunHttp("127.0.0.1:0"); err != nil {
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
