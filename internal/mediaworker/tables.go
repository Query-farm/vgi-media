// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"context"
	"strconv"

	"github.com/Query-farm/vgi-go/vgi"
	"github.com/Query-farm/vgi-rpc-go/vgirpc"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// allocator is the shared Go allocator for all array building in this package.
var allocator = memory.NewGoAllocator()

// executableExamples is a guaranteed-runnable, catalog-qualified set of examples
// (VGI509). Each `sql` is self-contained and re-runnable against an attached
// `media` worker. The examples probe the committed video fixture (resolved to an
// absolute path at startup) so every scalar returns a concrete value and the
// table functions return real rows. We deliberately omit `expected_result` — the
// linter only needs each query to execute and return data, and pinning exact
// codec/duration output would be brittle across ffprobe versions.
var executableExamples = buildExecutableExamples()

func buildExecutableExamples() string {
	// The path is embedded inside a single-quoted SQL literal which is itself a
	// JSON string: escape for SQL first, then for JSON.
	v := jsonEscape(sqlEscape(exampleVideoPath))
	return `[
  {
    "description": "Probe a media file's container format, duration, and stream count in one row.",
    "sql": "SELECT media.main.media_format('` + v + `') AS format, media.main.duration('` + v + `') AS seconds, media.main.stream_count('` + v + `') AS streams"
  },
  {
    "description": "Read the first video stream's codec, resolution, and frame rate.",
    "sql": "SELECT media.main.video_codec('` + v + `') AS vcodec, media.main.resolution('` + v + `') AS res, media.main.fps('` + v + `') AS fps"
  },
  {
    "description": "List every elementary stream in a media file, one row per stream.",
    "sql": "SELECT idx, type, codec FROM media.main.media_streams('` + v + `') ORDER BY idx"
  },
  {
    "description": "List the container-level metadata tags of a media file as key/value rows.",
    "sql": "SELECT key, value FROM media.main.media_tags('` + v + `') ORDER BY key"
  }
]`
}

// sqlEscape escapes a string for safe interpolation inside a single-quoted SQL
// string literal (doubles embedded single quotes).
func sqlEscape(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\'' {
			out = append(out, '\'')
		}
		out = append(out, r)
	}
	return string(out)
}

// jsonEscape escapes a string for safe interpolation inside a JSON string
// literal (handles backslashes and double quotes — the only characters that can
// occur in a filesystem path that would break the surrounding JSON).
func jsonEscape(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r == '\\' || r == '"' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(out)
}

func itoa(n int) string { return strconv.Itoa(n) }

// WHY AN EXPLICIT CURSOR, NOT A bool Done (the HTTP-continuation fix):
//
// Over the HTTP transport the worker is STATELESS across exchanges — there is no
// long-lived process holding the live state between Process ticks. The framework
// round-trips the producer state through an opaque continuation token: after each
// tick it gob-encodes the state (snapshotting the LIVE user state), the client
// returns the token, and the worker resumes by gob-decoding it. The HTTP server
// emits at most one data batch per response, so a producer with more to emit is
// always resumed mid-stream from its token.
//
// The position MUST therefore live in the serialized state. A bare `Done bool`
// flipped only AFTER the single Emit does not survive the continuation boundary:
// the resumed tick observes the pre-Emit snapshot, re-emits the same rows, and
// the scan never terminates (an infinite loop — subprocess/unix keep live state
// in memory, so they were unaffected and hid the bug). Carrying an explicit
// Offset that Process advances BEFORE yielding makes the snapshot authoritative.
//
// rowsPerTick bounds how many rows each Process tick emits, so the cursor is
// observable across the continuation boundary (and scales to large results).
const rowsPerTick = 256

// cursorBounds returns [start,end) for the next bounded slice over n rows
// starting at *offset, advancing *offset past it; done=true once all consumed.
func cursorBounds(n int, offset *int) (start, end int, done bool) {
	if *offset >= n {
		return 0, 0, true
	}
	start = *offset
	end = start + rowsPerTick
	if end > n {
		end = n
	}
	*offset = end
	return start, end, false
}

// tableArgs is the single-argument struct for the table functions: a path
// (VARCHAR) or media bytes (BLOB). NOTE: table functions cannot take a column
// arg (they have no streamed input batch), so the input is a CONST scalar
// argument here — read in NewState via params.Args.
type tableArgs struct {
	Input string `vgi:"pos=0,type=any,doc=Media file path (VARCHAR) or media bytes (BLOB)"`
}

// probeArg0 probes positional argument 0 of a table function, which may be a
// VARCHAR path or a BLOB. Returns (nil, nil) for NULL / non-media input, or
// ErrFFprobeNotFound if the binary is missing.
func probeArg0(ctx context.Context, args *vgi.Arguments) (*ProbeResult, error) {
	if args == nil {
		return nil, nil
	}
	col, err := args.GetColumn(0)
	if err != nil {
		return nil, nil
	}
	if col.Len() == 0 {
		return nil, nil
	}
	return probeRow(ctx, col, 0)
}

// ---------------------------------------------------------------------------
// media_streams(input) -> one row per elementary stream
// ---------------------------------------------------------------------------

var streamsSchema = arrow.NewSchema([]arrow.Field{
	{Name: "idx", Type: arrow.PrimitiveTypes.Int32},
	{Name: "type", Type: arrow.BinaryTypes.String},
	{Name: "codec", Type: arrow.BinaryTypes.String, Nullable: true},
	{Name: "width", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
	{Name: "height", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
	{Name: "bit_rate", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
	{Name: "duration", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
	{Name: "channels", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
	{Name: "sample_rate", Type: arrow.PrimitiveTypes.Int32, Nullable: true},
}, nil)

// streamRow is the gob-encodable, flattened form of one output row. The SDK
// gob-encodes table-function state between NewState and Process, so the state
// must contain ONLY exported, gob-safe Go values — never an arrow.Record or
// interface. Optionality is carried as explicit Has* bools.
type streamRow struct {
	Idx           int32
	Type          string
	Codec         string
	HasCodec      bool
	Width         int32
	HasWidth      bool
	Height        int32
	HasHeight     bool
	BitRate       int64
	HasBitRate    bool
	Duration      float64
	HasDuration   bool
	Channels      int32
	HasChannels   bool
	SampleRate    int32
	HasSampleRate bool
}

type streamsState struct {
	Rows   []streamRow
	Offset int
}

// StreamsFunction lists every elementary stream in the input media.
type StreamsFunction struct{}

var _ vgi.TypedTableFunc[streamsState] = (*StreamsFunction)(nil)

func (f *StreamsFunction) Name() string { return "media_streams" }
func (f *StreamsFunction) Metadata() vgi.FunctionMetadata {
	tags := objectTags(
		"List Media Streams",
		"List every elementary stream in a media container, one row per stream, with each "+
			"stream's index, type (video/audio/subtitle/data), codec, and the dimensions, bit "+
			"rate, duration, channel count, and sample rate that apply to it. The argument is a "+
			"file path (VARCHAR) or media bytes (BLOB); a non-media or missing input yields no rows.",
		"List every elementary stream (video/audio/subtitle/data) in a media file, one row "+
			"per stream. Columns: `idx`, `type`, `codec`, `width`, `height`, `bit_rate`, "+
			"`duration`, `channels`, `sample_rate`.",
		"media streams, streams, tracks, list streams, elementary streams, video stream, audio stream, "+
			"subtitle, codec per stream, stream table",
		"tables.go",
	)
	tags["vgi.columns_md"] = "| Column | Type | Description |\n" +
		"| --- | --- | --- |\n" +
		"| `idx` | INTEGER | Stream index within the container |\n" +
		"| `type` | VARCHAR | Stream codec type ('video', 'audio', 'subtitle', 'data') |\n" +
		"| `codec` | VARCHAR | Codec name, or NULL if unknown |\n" +
		"| `width` | INTEGER | Pixel width (video streams), or NULL |\n" +
		"| `height` | INTEGER | Pixel height (video streams), or NULL |\n" +
		"| `bit_rate` | BIGINT | Stream bit rate in bits per second, or NULL |\n" +
		"| `duration` | DOUBLE | Stream duration in seconds, or NULL |\n" +
		"| `channels` | INTEGER | Channel count (audio streams), or NULL |\n" +
		"| `sample_rate` | INTEGER | Sample rate in Hz (audio streams), or NULL |"
	tags["vgi.executable_examples"] = executableExamples
	return vgi.FunctionMetadata{
		Description: "One row per elementary stream (video/audio/subtitle/data) in the media",
		Examples: []vgi.CatalogExample{{
			SQL:         "SELECT * FROM media.main.media_streams('" + sqlEscape(exampleVideoPath) + "');",
			Description: "List every elementary stream (video/audio/subtitle/data) in a media file, one row per stream.",
		}},
		Stability:  vgi.StabilityConsistentWithinQuery,
		Categories: []string{"media"},
		Tags:       tags,
	}
}
func (f *StreamsFunction) ArgumentSpecs() []vgi.ArgSpec { return vgi.DeriveArgSpecs(tableArgs{}) }
func (f *StreamsFunction) OnBind(_ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindSchema(streamsSchema)
}
func (f *StreamsFunction) NewState(params *vgi.ProcessParams) (*streamsState, error) {
	r, err := probeArg0(context.Background(), params.Args)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &streamsState{}, nil
	}
	rows := make([]streamRow, 0, len(r.Streams))
	for i := range r.Streams {
		s := &r.Streams[i]
		row := streamRow{Idx: int32(s.Index), Type: s.CodecType}
		if s.CodecName != "" {
			row.Codec, row.HasCodec = s.CodecName, true
		}
		if s.Width > 0 {
			row.Width, row.HasWidth = int32(s.Width), true
		}
		if s.Height > 0 {
			row.Height, row.HasHeight = int32(s.Height), true
		}
		if v, ok := s.BitRateInt(); ok {
			row.BitRate, row.HasBitRate = v, true
		}
		if v, ok := s.DurationFloat(); ok {
			row.Duration, row.HasDuration = v, true
		}
		if s.Channels > 0 {
			row.Channels, row.HasChannels = int32(s.Channels), true
		}
		if v, ok := s.SampleRateInt(); ok {
			row.SampleRate, row.HasSampleRate = int32(v), true
		}
		rows = append(rows, row)
	}
	return &streamsState{Rows: rows}, nil
}
func (f *StreamsFunction) Process(_ context.Context, _ *vgi.ProcessParams, state *streamsState, out *vgirpc.OutputCollector) error {
	start, end, done := cursorBounds(len(state.Rows), &state.Offset)
	if done {
		return out.Finish()
	}
	rows := state.Rows[start:end]
	n := len(rows)

	idx := array.NewInt32Builder(allocator)
	typ := array.NewStringBuilder(allocator)
	codec := array.NewStringBuilder(allocator)
	width := array.NewInt32Builder(allocator)
	height := array.NewInt32Builder(allocator)
	bitRate := array.NewInt64Builder(allocator)
	duration := array.NewFloat64Builder(allocator)
	channels := array.NewInt32Builder(allocator)
	sampleRate := array.NewInt32Builder(allocator)
	for _, b := range []interface{ Reserve(int) }{idx, typ, codec, width, height, bitRate, duration, channels, sampleRate} {
		b.Reserve(n)
	}

	for _, r := range rows {
		idx.Append(r.Idx)
		typ.Append(r.Type)
		appendStr(codec, r.Codec, r.HasCodec)
		appendI32(width, r.Width, r.HasWidth)
		appendI32(height, r.Height, r.HasHeight)
		appendI64(bitRate, r.BitRate, r.HasBitRate)
		appendF64(duration, r.Duration, r.HasDuration)
		appendI32(channels, r.Channels, r.HasChannels)
		appendI32(sampleRate, r.SampleRate, r.HasSampleRate)
	}

	cols := []arrow.Array{
		idx.NewArray(), typ.NewArray(), codec.NewArray(), width.NewArray(),
		height.NewArray(), bitRate.NewArray(), duration.NewArray(),
		channels.NewArray(), sampleRate.NewArray(),
	}
	for _, b := range []interface{ Release() }{idx, typ, codec, width, height, bitRate, duration, channels, sampleRate} {
		b.Release()
	}
	batch := array.NewRecordBatch(streamsSchema, cols, int64(n))
	defer batch.Release()
	for _, c := range cols {
		c.Release()
	}
	return out.Emit(batch)
}

// NewStreamsFunction builds the registerable table function.
func NewStreamsFunction() vgi.TableFunction {
	return vgi.AsTableFunction[streamsState](&StreamsFunction{})
}

// ---------------------------------------------------------------------------
// media_tags(input) -> format-level metadata tags
// ---------------------------------------------------------------------------

var tagsSchema = arrow.NewSchema([]arrow.Field{
	{Name: "key", Type: arrow.BinaryTypes.String},
	{Name: "value", Type: arrow.BinaryTypes.String},
}, nil)

type tagKV struct {
	Key   string
	Value string
}

type tagsState struct {
	Tags   []tagKV
	Offset int
}

// TagsFunction lists the container-level (format) metadata tags.
type TagsFunction struct{}

var _ vgi.TypedTableFunc[tagsState] = (*TagsFunction)(nil)

func (f *TagsFunction) Name() string { return "media_tags" }
func (f *TagsFunction) Metadata() vgi.FunctionMetadata {
	tags := objectTags(
		"List Media Tags",
		"List the container-level (format) metadata tags of a media file as key/value rows, "+
			"e.g. title, artist, album, comment, encoder, creation_time. The argument is a file "+
			"path (VARCHAR) or media bytes (BLOB); a non-media or missing input yields no rows.",
		"List the container-level metadata tags (title, artist, encoder, ...) of a media file "+
			"as key/value rows. Columns: `key`, `value`.",
		"media tags, metadata, tags, title, artist, album, encoder, creation time, key value, "+
			"format metadata, comments",
		"tables.go",
	)
	tags["vgi.columns_md"] = "| Column | Type | Description |\n" +
		"| --- | --- | --- |\n" +
		"| `key` | VARCHAR | Metadata tag name (e.g. 'title', 'artist', 'encoder') |\n" +
		"| `value` | VARCHAR | Metadata tag value |"
	return vgi.FunctionMetadata{
		Description: "One row per format-level metadata tag (title, artist, encoder, ...)",
		Examples: []vgi.CatalogExample{{
			SQL:         "SELECT key, value FROM media.main.media_tags('" + sqlEscape(exampleVideoPath) + "');",
			Description: "List the container-level metadata tags (title, artist, encoder, ...) of a media file as key/value rows.",
		}},
		Stability:  vgi.StabilityConsistentWithinQuery,
		Categories: []string{"media"},
		Tags:       tags,
	}
}
func (f *TagsFunction) ArgumentSpecs() []vgi.ArgSpec { return vgi.DeriveArgSpecs(tableArgs{}) }
func (f *TagsFunction) OnBind(_ *vgi.BindParams) (*vgi.BindResponse, error) {
	return vgi.BindSchema(tagsSchema)
}
func (f *TagsFunction) NewState(params *vgi.ProcessParams) (*tagsState, error) {
	r, err := probeArg0(context.Background(), params.Args)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return &tagsState{}, nil
	}
	// Sort keys for deterministic output ordering.
	keys := make([]string, 0, len(r.Format.Tags))
	for k := range r.Format.Tags {
		keys = append(keys, k)
	}
	sortStrings(keys)
	tags := make([]tagKV, 0, len(keys))
	for _, k := range keys {
		tags = append(tags, tagKV{Key: k, Value: r.Format.Tags[k]})
	}
	return &tagsState{Tags: tags}, nil
}
func (f *TagsFunction) Process(_ context.Context, _ *vgi.ProcessParams, state *tagsState, out *vgirpc.OutputCollector) error {
	start, end, done := cursorBounds(len(state.Tags), &state.Offset)
	if done {
		return out.Finish()
	}
	t := state.Tags[start:end]
	n := int64(len(t))
	batch := array.NewRecordBatch(tagsSchema, []arrow.Array{
		vgi.BuildStringArray(n, func(i int64) string { return t[i].Key }),
		vgi.BuildStringArray(n, func(i int64) string { return t[i].Value }),
	}, n)
	defer batch.Release()
	return out.Emit(batch)
}

// NewTagsFunction builds the registerable table function.
func NewTagsFunction() vgi.TableFunction {
	return vgi.AsTableFunction[tagsState](&TagsFunction{})
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func appendStr(b *array.StringBuilder, v string, ok bool) {
	if ok {
		b.Append(v)
	} else {
		b.AppendNull()
	}
}
func appendI32(b *array.Int32Builder, v int32, ok bool) {
	if ok {
		b.Append(v)
	} else {
		b.AppendNull()
	}
}
func appendI64(b *array.Int64Builder, v int64, ok bool) {
	if ok {
		b.Append(v)
	} else {
		b.AppendNull()
	}
}
func appendF64(b *array.Float64Builder, v float64, ok bool) {
	if ok {
		b.Append(v)
	} else {
		b.AppendNull()
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// Register registers all media scalar and table functions on the worker.
func Register(w *vgi.Worker) {
	registerScalars(w)
	w.RegisterTable(NewStreamsFunction())
	w.RegisterTable(NewTagsFunction())
}
