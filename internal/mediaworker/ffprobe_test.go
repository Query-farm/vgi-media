// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// fixture returns the absolute path to a committed test fixture under
// test/sql/data (two levels up from this package).
func fixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "test", "sql", "data", name))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %s missing: %v (run `make fixtures`)", name, err)
	}
	return p
}

func TestProbePathMP4(t *testing.T) {
	r, err := ProbePath(context.Background(), fixture(t, "tiny.mp4"))
	if err != nil {
		t.Fatalf("ProbePath: %v", err)
	}
	if r.Format.FormatName != "mov,mp4,m4a,3gp,3g2,mj2" {
		t.Errorf("format_name = %q", r.Format.FormatName)
	}
	if d, ok := r.Duration(); !ok || d < 0.5 || d > 1.5 {
		t.Errorf("duration = %v (ok=%v), want ~1.0", d, ok)
	}
	v, ok := r.FirstVideo()
	if !ok {
		t.Fatal("expected a video stream")
	}
	if v.CodecName != "h264" {
		t.Errorf("video codec = %q, want h264", v.CodecName)
	}
	if v.Width != 320 || v.Height != 240 {
		t.Errorf("resolution = %dx%d, want 320x240", v.Width, v.Height)
	}
	if fps, ok := v.FPS(); !ok || fps != 10 {
		t.Errorf("fps = %v (ok=%v), want 10", fps, ok)
	}
}

func TestProbePathWAV(t *testing.T) {
	r, err := ProbePath(context.Background(), fixture(t, "silent.wav"))
	if err != nil {
		t.Fatalf("ProbePath: %v", err)
	}
	if r.Format.FormatName != "wav" {
		t.Errorf("format_name = %q, want wav", r.Format.FormatName)
	}
	a, ok := r.FirstAudio()
	if !ok {
		t.Fatal("expected an audio stream")
	}
	if a.CodecName != "pcm_s16le" {
		t.Errorf("audio codec = %q", a.CodecName)
	}
	if sr, ok := a.SampleRateInt(); !ok || sr != 8000 {
		t.Errorf("sample_rate = %v (ok=%v), want 8000", sr, ok)
	}
	if a.Channels != 1 {
		t.Errorf("channels = %d, want 1", a.Channels)
	}
	if _, ok := r.FirstVideo(); ok {
		t.Error("WAV should have no video stream")
	}
}

func TestProbeBytesMP4(t *testing.T) {
	data, err := os.ReadFile(fixture(t, "tiny.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := ProbeBytes(context.Background(), data)
	if err != nil {
		t.Fatalf("ProbeBytes: %v", err)
	}
	if v, ok := r.FirstVideo(); !ok || v.Width != 320 {
		t.Errorf("expected 320-wide video from piped bytes, got %v ok=%v", v, ok)
	}
}

func TestProbeGarbageBytes(t *testing.T) {
	// Random non-media bytes: ffprobe must reject them and we must NOT crash.
	garbage := []byte("this is definitely not a media file \x00\x01\x02\xff\xfe")
	_, err := ProbeBytes(context.Background(), garbage)
	if err == nil {
		t.Fatal("expected an error probing garbage bytes")
	}
	if err == ErrFFprobeNotFound {
		t.Fatal("garbage input must not look like a missing-binary error")
	}
}

func TestProbeMissingBinary(t *testing.T) {
	orig := ffprobeBinary
	ffprobeBinary = filepath.Join(t.TempDir(), "no-such-ffprobe")
	defer func() { ffprobeBinary = orig }()

	_, err := ProbePath(context.Background(), fixture(t, "tiny.mp4"))
	if err != ErrFFprobeNotFound {
		t.Fatalf("expected ErrFFprobeNotFound, got %v", err)
	}
}

func TestParseRate(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"30/1", 30, true},
		{"30000/1001", 30000.0 / 1001.0, true},
		{"0/0", 0, false},
		{"", 0, false},
		{"N/A", 0, false},
		{"24", 24, true},
	}
	for _, c := range cases {
		got, ok := parseRate(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("parseRate(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}
