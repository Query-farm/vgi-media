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
	Input arrow.Array `vgi:"pos=0,const=false,doc=Media file path (VARCHAR) or media bytes (BLOB)"`
}

// scalarFn is the per-row probe→value logic. Given a decoded ProbeResult it
// returns the typed value and whether it is present (false → SQL NULL).
//
// The generic emit machinery lives in mapProbe* below; each concrete scalar is
// just a name + a closure.

// stringScalar implements a VARCHAR-returning scalar over a probed input.
type stringScalar struct {
	name string
	desc string
	get  func(r *ProbeResult) (string, bool)
}

func (f *stringScalar) Name() string { return f.name }
func (f *stringScalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Stability: vgi.StabilityVolatile, Categories: []string{"media"}}
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
	name string
	desc string
	get  func(r *ProbeResult) (int64, bool)
}

func (f *int64Scalar) Name() string { return f.name }
func (f *int64Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Stability: vgi.StabilityVolatile, Categories: []string{"media"}}
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
	name string
	desc string
	get  func(r *ProbeResult) (int32, bool)
}

func (f *int32Scalar) Name() string { return f.name }
func (f *int32Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Stability: vgi.StabilityVolatile, Categories: []string{"media"}}
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
	name string
	desc string
	get  func(r *ProbeResult) (float64, bool)
}

func (f *float64Scalar) Name() string { return f.name }
func (f *float64Scalar) Metadata() vgi.FunctionMetadata {
	return vgi.FunctionMetadata{Description: f.desc, Stability: vgi.StabilityVolatile, Categories: []string{"media"}}
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

// registerScalars registers every scalar function on the worker.
func registerScalars(w *vgi.Worker) {
	// --- container-level ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		name: "media_format", desc: "Container format name (format_name), e.g. 'mov,mp4,m4a,3gp,3g2,mj2'",
		get: func(r *ProbeResult) (string, bool) {
			if r.Format.FormatName == "" {
				return "", false
			}
			return r.Format.FormatName, true
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&float64Scalar{
		name: "duration", desc: "Container duration in seconds",
		get: func(r *ProbeResult) (float64, bool) { return r.Duration() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int64Scalar{
		name: "bitrate", desc: "Container bit rate in bits per second",
		get: func(r *ProbeResult) (int64, bool) { return r.BitRate() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int64Scalar{
		name: "media_size", desc: "Container size in bytes",
		get: func(r *ProbeResult) (int64, bool) { return r.Size() },
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		name: "stream_count", desc: "Number of elementary streams in the container",
		get: func(r *ProbeResult) (int32, bool) { return int32(len(r.Streams)), true },
	}))

	// --- video ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		name: "video_codec", desc: "Codec name of the first video stream",
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstVideo(); ok && s.CodecName != "" {
				return s.CodecName, true
			}
			return "", false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		name: "width", desc: "Pixel width of the first video stream",
		get: func(r *ProbeResult) (int32, bool) {
			if s, ok := r.FirstVideo(); ok && s.Width > 0 {
				return int32(s.Width), true
			}
			return 0, false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&int32Scalar{
		name: "height", desc: "Pixel height of the first video stream",
		get: func(r *ProbeResult) (int32, bool) {
			if s, ok := r.FirstVideo(); ok && s.Height > 0 {
				return int32(s.Height), true
			}
			return 0, false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		name: "resolution", desc: "Resolution of the first video stream as 'WIDTHxHEIGHT' (e.g. '1920x1080')",
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstVideo(); ok && s.Width > 0 && s.Height > 0 {
				return itoa(s.Width) + "x" + itoa(s.Height), true
			}
			return "", false
		},
	}))
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&float64Scalar{
		name: "fps", desc: "Frames per second of the first video stream (from avg_frame_rate)",
		get: func(r *ProbeResult) (float64, bool) {
			if s, ok := r.FirstVideo(); ok {
				return s.FPS()
			}
			return 0, false
		},
	}))

	// --- audio ---
	w.RegisterScalar(vgi.AsScalarFunction[inputArg](&stringScalar{
		name: "audio_codec", desc: "Codec name of the first audio stream",
		get: func(r *ProbeResult) (string, bool) {
			if s, ok := r.FirstAudio(); ok && s.CodecName != "" {
				return s.CodecName, true
			}
			return "", false
		},
	}))
}
