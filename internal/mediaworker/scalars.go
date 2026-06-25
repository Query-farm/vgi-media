// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"context"

	"github.com/Query-farm/vgi-go/vgi"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// inputArg is the shared single-argument struct for every scalar. The input is
// a column arg (`const=false`) declared as arrow.Array so it accepts EITHER a
// VARCHAR path OR a BLOB of media bytes — DeriveArgSpecs advertises it as
// arrow_type="any". The function body reads the raw column directly.
type inputArg struct {
	Input arrow.Array `vgi:"pos=0,const=false,doc=The media to probe: either a filesystem path to a media file or the raw media bytes themselves; ffprobe reads it to extract the requested metadata"`
}

// scalarFn is the per-row probe→value logic. Given a decoded ProbeResult it
// returns the typed value and whether it is present (false → SQL NULL).
//
// The generic emit machinery lives in mapProbe* below; each concrete scalar is
// just a name + a closure.

// scalarMeta carries the per-object discovery/description metadata (VGI112/113/
// 124/126/128) every scalar must surface. It is embedded in each typed scalar
// shape so the Metadata() builders share one tag-assembly path.
type scalarMeta struct {
	title    string
	llm      string
	md       string
	keywords string
}

// tags assembles the five standard per-object tags for a scalar, all backed by
// scalars.go.
func (m scalarMeta) tags() map[string]string {
	return objectTags(m.title, m.llm, m.md, m.keywords, "scalars.go")
}

// scalarStability is the stability advertised by every media scalar. Probing a
// file is CONSISTENT_WITHIN_QUERY: ffprobe returns the same result for the same
// path within a single query, but the result can change across queries if the
// underlying file is replaced. That is the honest classification — not VOLATILE
// (which would suppress all engine caching) and not CONSISTENT (the file is an
// external, mutable resource).
const scalarStability = vgi.StabilityConsistentWithinQuery

// stringScalar implements a VARCHAR-returning scalar over a probed input.
type stringScalar struct {
	scalarMeta
	name     string
	desc     string
	examples []vgi.CatalogExample
	get      func(r *ProbeResult) (string, bool)
}

func (f *stringScalar) Name() string { return f.name }
func (f *stringScalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Examples: f.examples, Stability: scalarStability, Categories: []string{"media"}, Tags: f.tags()}
}
func (f *stringScalar) OnBindTyped(_ *inputArg, _ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindResult(arrow.BinaryTypes.String)
}
func (f *stringScalar) ProcessTyped(ctx context.Context, _ *inputArg, params *vgi.ProcessParams, batch arrow.RecordBatch) (arrow.RecordBatch, error) {
	col := batch.Column(0)
	n := int(batch.NumRows())
	b := array.NewStringBuilder(allocator)
	defer b.Release()
	b.Reserve(n)
	for i := 0; i < n; i++ {
		r, err := probeRow(ctx, col, i)
		if err != nil {
			return nil, err
		}
		if v, ok := getOpt(r, f.get); ok {
			b.Append(v)
		} else {
			b.AppendNull()
		}
	}
	arr := b.NewArray()
	defer arr.Release()
	return array.NewRecordBatch(params.OutputSchema, []arrow.Array{arr}, int64(n)), nil
}

// int64Scalar implements a BIGINT-returning scalar over a probed input.
type int64Scalar struct {
	scalarMeta
	name     string
	desc     string
	examples []vgi.CatalogExample
	get      func(r *ProbeResult) (int64, bool)
}

func (f *int64Scalar) Name() string { return f.name }
func (f *int64Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Examples: f.examples, Stability: scalarStability, Categories: []string{"media"}, Tags: f.tags()}
}
func (f *int64Scalar) OnBindTyped(_ *inputArg, _ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindResult(arrow.PrimitiveTypes.Int64)
}
func (f *int64Scalar) ProcessTyped(ctx context.Context, _ *inputArg, params *vgi.ProcessParams, batch arrow.RecordBatch) (arrow.RecordBatch, error) {
	col := batch.Column(0)
	n := int(batch.NumRows())
	b := array.NewInt64Builder(allocator)
	defer b.Release()
	b.Reserve(n)
	for i := 0; i < n; i++ {
		r, err := probeRow(ctx, col, i)
		if err != nil {
			return nil, err
		}
		if v, ok := getOpt(r, f.get); ok {
			b.Append(v)
		} else {
			b.AppendNull()
		}
	}
	arr := b.NewArray()
	defer arr.Release()
	return array.NewRecordBatch(params.OutputSchema, []arrow.Array{arr}, int64(n)), nil
}

// int32Scalar implements an INTEGER-returning scalar over a probed input.
type int32Scalar struct {
	scalarMeta
	name     string
	desc     string
	examples []vgi.CatalogExample
	get      func(r *ProbeResult) (int32, bool)
}

func (f *int32Scalar) Name() string { return f.name }
func (f *int32Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Examples: f.examples, Stability: scalarStability, Categories: []string{"media"}, Tags: f.tags()}
}
func (f *int32Scalar) OnBindTyped(_ *inputArg, _ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindResult(arrow.PrimitiveTypes.Int32)
}
func (f *int32Scalar) ProcessTyped(ctx context.Context, _ *inputArg, params *vgi.ProcessParams, batch arrow.RecordBatch) (arrow.RecordBatch, error) {
	col := batch.Column(0)
	n := int(batch.NumRows())
	b := array.NewInt32Builder(allocator)
	defer b.Release()
	b.Reserve(n)
	for i := 0; i < n; i++ {
		r, err := probeRow(ctx, col, i)
		if err != nil {
			return nil, err
		}
		if v, ok := getOpt(r, f.get); ok {
			b.Append(v)
		} else {
			b.AppendNull()
		}
	}
	arr := b.NewArray()
	defer arr.Release()
	return array.NewRecordBatch(params.OutputSchema, []arrow.Array{arr}, int64(n)), nil
}

// float64Scalar implements a DOUBLE-returning scalar over a probed input.
type float64Scalar struct {
	scalarMeta
	name     string
	desc     string
	examples []vgi.CatalogExample
	get      func(r *ProbeResult) (float64, bool)
}

func (f *float64Scalar) Name() string { return f.name }
func (f *float64Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Examples: f.examples, Stability: scalarStability, Categories: []string{"media"}, Tags: f.tags()}
}
func (f *float64Scalar) OnBindTyped(_ *inputArg, _ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindResult(arrow.PrimitiveTypes.Float64)
}
func (f *float64Scalar) ProcessTyped(ctx context.Context, _ *inputArg, params *vgi.ProcessParams, batch arrow.RecordBatch) (arrow.RecordBatch, error) {
	col := batch.Column(0)
	n := int(batch.NumRows())
	b := array.NewFloat64Builder(allocator)
	defer b.Release()
	b.Reserve(n)
	for i := 0; i < n; i++ {
		r, err := probeRow(ctx, col, i)
		if err != nil {
			return nil, err
		}
		if v, ok := getOpt(r, f.get); ok {
			b.Append(v)
		} else {
			b.AppendNull()
		}
	}
	arr := b.NewArray()
	defer arr.Release()
	return array.NewRecordBatch(params.OutputSchema, []arrow.Array{arr}, int64(n)), nil
}

// getOpt applies get to r, returning (zero, false) when r is nil (NULL input or
// probe failure) so every accessor closure can assume a non-nil result.
func getOpt[T any](r *ProbeResult, get func(*ProbeResult) (T, bool)) (T, bool) {
	if r == nil {
		var zero T
		return zero, false
	}
	return get(r)
}

// ex builds a single catalog-qualified example for a scalar. The SQL is always
// qualified as media.main.<fn>(...) so the metadata linter sees a fully
// resolvable, copy-pasteable query referencing the function by name.
func ex(sql, desc string) []vgi.CatalogExample {
	return []vgi.CatalogExample{{SQL: sql, Description: desc}}
}

// registerScalars registers every scalar function on the worker.
func registerScalars(w *vgi.Worker) {
	// --- container-level ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		scalarMeta: scalarMeta{
			title: "Container Format Name",
			llm: "Return the container/wrapper format name of a media file, as reported by " +
				"ffprobe's format_name (e.g. 'mov,mp4,m4a,3gp,3g2,mj2' for an MP4, 'matroska,webm' " +
				"for an MKV, 'wav' for a WAV). The argument is a file path (VARCHAR) or media bytes " +
				"(BLOB); returns NULL when the input is not decodable media.",
			md: "Return the container format name of a media file, e.g. " +
				"`media_format('/clips/intro.mp4')` → `mov,mp4,m4a,3gp,3g2,mj2`.",
			keywords: "media format, container format, format_name, wrapper, mp4, mkv, wav, mov, container type",
		},
		name: "media_format", desc: "Container format name (format_name), e.g. 'mov,mp4,m4a,3gp,3g2,mj2'",
		examples: ex(
			"SELECT media.main.media_format('/clips/intro.mp4');",
			"Return the container format name of a media file given its path.",
		),
		get: func(r *ProbeResult) (string, bool) {
			if r.Format.FormatName == "" {
				return "", false
			}
			return r.Format.FormatName, true
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&float64Scalar{
		scalarMeta: scalarMeta{
			title: "Media Duration Seconds",
			llm: "Return the total playback duration of a media file in seconds (DOUBLE), taken " +
				"from the container format header. The argument is a file path (VARCHAR) or media " +
				"bytes (BLOB); returns NULL when ffprobe cannot determine a duration.",
			md: "Return the duration of a media file in seconds, e.g. " +
				"`duration('/clips/intro.mp4')` → `12.5`.",
			keywords: "duration, length, runtime, seconds, playback time, how long, media length",
		},
		name: "duration", desc: "Container duration in seconds",
		examples: ex(
			"SELECT media.main.duration('/clips/intro.mp4');",
			"Return the duration of a media file in seconds.",
		),
		get: func(r *ProbeResult) (float64, bool) { return r.Duration() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int64Scalar{
		scalarMeta: scalarMeta{
			title: "Overall Bit Rate",
			llm: "Return the overall container bit rate of a media file in bits per second " +
				"(BIGINT), as reported by ffprobe's format bit_rate. The argument is a file path " +
				"(VARCHAR) or media bytes (BLOB); returns NULL when the bit rate is unknown.",
			md: "Return the overall container bit rate in bits per second, e.g. " +
				"`bitrate('/clips/intro.mp4')` → `2500000`.",
			keywords: "bitrate, bit rate, bits per second, bps, data rate, quality, encoding rate",
		},
		name: "bitrate", desc: "Container bit rate in bits per second",
		examples: ex(
			"SELECT media.main.bitrate('/clips/intro.mp4');",
			"Return the overall container bit rate in bits per second.",
		),
		get: func(r *ProbeResult) (int64, bool) { return r.BitRate() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int64Scalar{
		scalarMeta: scalarMeta{
			title: "Media File Size Bytes",
			llm: "Return the size of a media file in bytes (BIGINT), as reported by ffprobe's " +
				"format size field. The argument is a file path (VARCHAR) or media bytes (BLOB); " +
				"returns NULL when ffprobe does not report a size (e.g. some piped BLOB inputs).",
			md: "Return the size of a media file in bytes, e.g. " +
				"`media_size('/clips/intro.mp4')` → `4194304`.",
			keywords: "size, file size, bytes, media size, length in bytes, byte count, file weight",
		},
		name: "media_size", desc: "Container size in bytes",
		examples: ex(
			"SELECT media.main.media_size('/clips/intro.mp4');",
			"Return the size of a media file in bytes as reported by ffprobe.",
		),
		get: func(r *ProbeResult) (int64, bool) { return r.Size() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		scalarMeta: scalarMeta{
			title: "Elementary Stream Count",
			llm: "Return the number of elementary streams in a media container (INTEGER), counting " +
				"video, audio, subtitle, and data streams. The argument is a file path (VARCHAR) or " +
				"media bytes (BLOB); returns NULL when the input is not decodable media.",
			md: "Count the elementary streams in a media file, e.g. " +
				"`stream_count('/clips/intro.mp4')` → `2`.",
			keywords: "stream count, number of streams, tracks, how many streams, elementary streams, track count",
		},
		name: "stream_count", desc: "Number of elementary streams in the container",
		examples: ex(
			"SELECT media.main.stream_count('/clips/intro.mp4');",
			"Count the elementary streams (video/audio/subtitle/data) in a media file.",
		),
		get: func(r *ProbeResult) (int32, bool) { return int32(len(r.Streams)), true },
	}))

	// --- video ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		scalarMeta: scalarMeta{
			title: "First Video Codec",
			llm: "Return the codec name of the first video stream in a media file (e.g. 'h264', " +
				"'hevc', 'vp9', 'av1'), as reported by ffprobe. The argument is a file path " +
				"(VARCHAR) or media bytes (BLOB); returns NULL when the file has no video stream.",
			md: "Return the codec name of the first video stream, e.g. " +
				"`video_codec('/clips/intro.mp4')` → `h264`.",
			keywords: "video codec, codec, h264, hevc, h265, vp9, av1, video encoding, video format",
		},
		name: "video_codec", desc: "Codec name of the first video stream",
		examples: ex(
			"SELECT media.main.video_codec('/clips/intro.mp4');",
			"Return the codec name of the first video stream (e.g. 'h264').",
		),
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstVideo(); ok && s.CodecName != "" {
				return s.CodecName, true
			}
			return "", false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		scalarMeta: scalarMeta{
			title: "Video Pixel Width",
			llm: "Return the pixel width of the first video stream in a media file (INTEGER), as " +
				"reported by ffprobe. The argument is a file path (VARCHAR) or media bytes (BLOB); " +
				"returns NULL when the file has no video stream.",
			md: "Return the pixel width of the first video stream, e.g. " +
				"`width('/clips/intro.mp4')` → `1920`.",
			keywords: "width, pixel width, horizontal resolution, video width, frame width, pixels wide",
		},
		name: "width", desc: "Pixel width of the first video stream",
		examples: ex(
			"SELECT media.main.width('/clips/intro.mp4');",
			"Return the pixel width of the first video stream.",
		),
		get: func(r *ProbeResult) (int32, bool) {
			if s, ok := r.FirstVideo(); ok && s.Width > 0 {
				return int32(s.Width), true
			}
			return 0, false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		scalarMeta: scalarMeta{
			title: "Video Pixel Height",
			llm: "Return the pixel height of the first video stream in a media file (INTEGER), as " +
				"reported by ffprobe. The argument is a file path (VARCHAR) or media bytes (BLOB); " +
				"returns NULL when the file has no video stream.",
			md: "Return the pixel height of the first video stream, e.g. " +
				"`height('/clips/intro.mp4')` → `1080`.",
			keywords: "height, pixel height, vertical resolution, video height, frame height, pixels tall",
		},
		name: "height", desc: "Pixel height of the first video stream",
		examples: ex(
			"SELECT media.main.height('/clips/intro.mp4');",
			"Return the pixel height of the first video stream.",
		),
		get: func(r *ProbeResult) (int32, bool) {
			if s, ok := r.FirstVideo(); ok && s.Height > 0 {
				return int32(s.Height), true
			}
			return 0, false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		scalarMeta: scalarMeta{
			title: "Video Resolution String",
			llm: "Return the resolution of the first video stream formatted as 'WIDTHxHEIGHT' " +
				"(e.g. '1920x1080', '3840x2160'). The argument is a file path (VARCHAR) or media " +
				"bytes (BLOB); returns NULL when the file has no video stream with known dimensions.",
			md: "Return the resolution of the first video stream as `WIDTHxHEIGHT`, e.g. " +
				"`resolution('/clips/intro.mp4')` → `1920x1080`.",
			keywords: "resolution, dimensions, widthxheight, 1080p, 4k, 720p, frame size, video size",
		},
		name: "resolution", desc: "Resolution of the first video stream as 'WIDTHxHEIGHT' (e.g. '1920x1080')",
		examples: ex(
			"SELECT media.main.resolution('/clips/intro.mp4');",
			"Return the resolution of the first video stream as 'WIDTHxHEIGHT'.",
		),
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstVideo(); ok && s.Width > 0 && s.Height > 0 {
				return itoa(s.Width) + "x" + itoa(s.Height), true
			}
			return "", false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&float64Scalar{
		scalarMeta: scalarMeta{
			title: "Video Frame Rate",
			llm: "Return the average frame rate (frames per second, DOUBLE) of the first video " +
				"stream, computed from ffprobe's avg_frame_rate fraction. The argument is a file " +
				"path (VARCHAR) or media bytes (BLOB); returns NULL when the file has no video " +
				"stream or the frame rate is unknown.",
			md: "Return the average frame rate (fps) of the first video stream, e.g. " +
				"`fps('/clips/intro.mp4')` → `29.97`.",
			keywords: "fps, frame rate, frames per second, framerate, avg_frame_rate, 30fps, 60fps, frame timing",
		},
		name: "fps", desc: "Frames per second of the first video stream (from avg_frame_rate)",
		examples: ex(
			"SELECT media.main.fps('/clips/intro.mp4');",
			"Return the average frame rate (fps) of the first video stream.",
		),
		get: func(r *ProbeResult) (float64, bool) {
			if s, ok := r.FirstVideo(); ok {
				return s.FPS()
			}
			return 0, false
		},
	}))

	// --- audio ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		scalarMeta: scalarMeta{
			title: "First Audio Codec",
			llm: "Return the codec name of the first audio stream in a media file (e.g. 'aac', " +
				"'mp3', 'opus', 'flac', 'pcm_s16le'), as reported by ffprobe. The argument is a " +
				"file path (VARCHAR) or media bytes (BLOB); returns NULL when the file has no " +
				"audio stream.",
			md: "Return the codec name of the first audio stream, e.g. " +
				"`audio_codec('/clips/intro.mp4')` → `aac`.",
			keywords: "audio codec, codec, aac, mp3, opus, flac, pcm, audio encoding, audio format, sound codec",
		},
		name: "audio_codec", desc: "Codec name of the first audio stream",
		examples: ex(
			"SELECT media.main.audio_codec('/clips/intro.mp4');",
			"Return the codec name of the first audio stream (e.g. 'aac').",
		),
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstAudio(); ok && s.CodecName != "" {
				return s.CodecName, true
			}
			return "", false
		},
	}))
}
