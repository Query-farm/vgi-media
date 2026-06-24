// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"bytes"
	"context"
	"encoding/gob"
	"testing"

	"github.com/Query-farm/vgi-go/vgi"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// strCol builds a 1-row string array (optionally NULL).
func strCol(v string, null bool) arrow.Array {
	b := array.NewStringBuilder(memory.DefaultAllocator)
	defer b.Release()
	if null {
		b.AppendNull()
	} else {
		b.Append(v)
	}
	return b.NewArray()
}

func argsWith(positional ...arrow.Array) *vgi.Arguments {
	return &vgi.Arguments{Positional: positional, Named: map[string]arrow.Array{}}
}

func TestStreamsNewStateMP4(t *testing.T) {
	f := &StreamsFunction{}
	st, err := f.NewState(&vgi.ProcessParams{
		Args: argsWith(strCol(fixture(t, "tiny.mp4"), false)),
	})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if len(st.Rows) != 1 {
		t.Fatalf("expected 1 stream row, got %d", len(st.Rows))
	}
	r := st.Rows[0]
	if r.Type != "video" || r.Codec != "h264" {
		t.Errorf("row = %+v, want video/h264", r)
	}
	if !r.HasWidth || r.Width != 320 || r.Height != 240 {
		t.Errorf("dims = %dx%d (hasW=%v)", r.Width, r.Height, r.HasWidth)
	}
	if st.Offset != 0 {
		t.Error("state cursor should start at offset 0 before Process")
	}
}

func TestStreamsNewStateWAV(t *testing.T) {
	f := &StreamsFunction{}
	st, err := f.NewState(&vgi.ProcessParams{
		Args: argsWith(strCol(fixture(t, "silent.wav"), false)),
	})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if len(st.Rows) != 1 {
		t.Fatalf("expected 1 audio stream, got %d", len(st.Rows))
	}
	r := st.Rows[0]
	if r.Type != "audio" {
		t.Errorf("type = %q, want audio", r.Type)
	}
	if !r.HasChannels || r.Channels != 1 {
		t.Errorf("channels = %d (has=%v), want 1", r.Channels, r.HasChannels)
	}
	if !r.HasSampleRate || r.SampleRate != 8000 {
		t.Errorf("sample_rate = %d (has=%v), want 8000", r.SampleRate, r.HasSampleRate)
	}
	// Audio stream: width/height must be NULL.
	if r.HasWidth || r.HasHeight {
		t.Error("audio stream should have NULL width/height")
	}
}

func TestStreamsNullInputNoRows(t *testing.T) {
	f := &StreamsFunction{}
	st, err := f.NewState(&vgi.ProcessParams{
		Args: argsWith(strCol("", true)),
	})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	if len(st.Rows) != 0 {
		t.Errorf("NULL input should yield no rows, got %d", len(st.Rows))
	}
}

func TestStreamsGarbagePathNoRows(t *testing.T) {
	f := &StreamsFunction{}
	st, err := f.NewState(&vgi.ProcessParams{
		Args: argsWith(strCol("/nonexistent/path/to/nothing.bin", false)),
	})
	if err != nil {
		t.Fatalf("NewState should swallow probe failure: %v", err)
	}
	if len(st.Rows) != 0 {
		t.Errorf("garbage path should yield no rows, got %d", len(st.Rows))
	}
}

func TestTagsNewStateMP4(t *testing.T) {
	f := &TagsFunction{}
	st, err := f.NewState(&vgi.ProcessParams{
		Args: argsWith(strCol(fixture(t, "tiny.mp4"), false)),
	})
	if err != nil {
		t.Fatalf("NewState: %v", err)
	}
	var hasEncoder bool
	for _, tag := range st.Tags {
		if tag.Key == "encoder" && tag.Value != "" {
			hasEncoder = true
		}
	}
	if !hasEncoder {
		t.Errorf("expected an 'encoder' tag, got %+v", st.Tags)
	}
}

// TestProbeRowGarbageBytes drives a BLOB column of garbage through probeRow:
// the worker must survive and report no result (nil, nil).
func TestProbeRowGarbageBytes(t *testing.T) {
	b := array.NewBinaryBuilder(memory.DefaultAllocator, arrow.BinaryTypes.Binary)
	defer b.Release()
	b.Append([]byte("\x00\x01not media\xff"))
	col := b.NewArray()
	defer col.Release()

	r, err := probeRow(context.Background(), col, 0)
	if err != nil {
		t.Fatalf("garbage bytes must not error out of probeRow: %v", err)
	}
	if r != nil {
		t.Errorf("garbage bytes should yield nil result, got %+v", r)
	}
}

// TestCursorSurvivesContinuation mirrors the HTTP transport: the per-scan state
// is gob round-tripped between ticks, so the cursor offset must advance across
// the boundary and eventually drain. A bare Done flag flipped after Emit would
// re-emit row 0 forever; the explicit Offset terminates.
func TestCursorSurvivesContinuation(t *testing.T) {
	n := rowsPerTick*2 + 5 // spans 3 ticks
	st := &streamsState{Rows: make([]streamRow, n)}
	emitted := 0
	for tick := 0; tick < 100; tick++ {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(st); err != nil {
			t.Fatalf("gob encode: %v", err)
		}
		var resumed streamsState
		if err := gob.NewDecoder(&buf).Decode(&resumed); err != nil {
			t.Fatalf("gob decode: %v", err)
		}
		st = &resumed
		start, end, done := cursorBounds(len(st.Rows), &st.Offset)
		if done {
			if emitted != n {
				t.Fatalf("drained after emitting %d of %d rows", emitted, n)
			}
			return
		}
		emitted += end - start
	}
	t.Fatal("cursor never drained — continuation loop did not terminate")
}
