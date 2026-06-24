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
			"source": "vgi-media",
			"vgi.description_llm": "Extract video, audio, and container metadata from media files " +
				"with ffprobe (ffmpeg). Scalars take a file path (VARCHAR) or media bytes (BLOB) and " +
				"return container format, duration, bit rate, size, stream count, and per-stream video " +
				"(codec, width, height, resolution, fps) and audio (codec) attributes. Table functions " +
				"list every elementary stream (media_streams) and every format-level metadata tag " +
				"(media_tags). Use for media inventory, transcoding triage, and quality/conformance checks in SQL.",
			"vgi.description_md": "# media\n\n" +
				"Video / audio / container metadata extraction over Apache Arrow, backed by " +
				"[`ffprobe`](https://ffmpeg.org/ffprobe.html).\n\n" +
				"Scalars accept a file path (VARCHAR) or media bytes (BLOB): `media_format`, `duration`, " +
				"`bitrate`, `media_size`, `stream_count`, `video_codec`, `width`, `height`, `resolution`, " +
				"`fps`, `audio_codec`.\n\n" +
				"Table functions: `media_streams` (one row per elementary stream), `media_tags` " +
				"(one row per format-level metadata tag).",
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
				"vgi.description_llm": "Media metadata functions: container-level scalars (format, " +
					"duration, bitrate, size, stream_count), per-stream video/audio scalars (codec, " +
					"width, height, resolution, fps), and table functions for elementary streams " +
					"(media_streams) and format-level metadata tags (media_tags).",
				"vgi.description_md": "Media metadata extraction functions (scalars + table functions) " +
					"over Apache Arrow, backed by ffprobe.",
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
