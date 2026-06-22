// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ffprobeBinary is the executable invoked to probe media. It is a package
// variable (not a const) so tests can point it at a non-existent / fake binary
// to exercise the "ffprobe missing" error path. In production it resolves
// against PATH — ffprobe (from ffmpeg) is a heavyweight NATIVE dependency that
// must be installed separately (Homebrew: `brew install ffmpeg`).
var ffprobeBinary = "ffprobe"

// probeTimeout bounds every ffprobe subprocess. Untrusted / truncated / garbage
// media must never hang or crash the worker, so the child process is killed if
// it exceeds this deadline. ffprobe itself is also capped with -analyzeduration
// / -probesize so it does not read unbounded input.
const probeTimeout = 15 * time.Second

// maxProbeBytes caps how much of a probesize ffprobe scans, bounding work on
// adversarial inputs. Tiny fixtures are well under this.
const maxProbeBytes = "16M"

// ProbeResult is the decoded subset of `ffprobe -show_format -show_streams`
// JSON output. Only the fields the worker exposes are modeled; everything else
// is ignored. All numeric fields arrive from ffprobe as JSON strings, so they
// are parsed lazily by the accessor helpers below.
type ProbeResult struct {
	Format  Format   `json:"format"`
	Streams []Stream `json:"streams"`
}

// Format is the container-level metadata block (`-show_format`).
type Format struct {
	Filename       string            `json:"filename"`
	NbStreams      int               `json:"nb_streams"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Duration       string            `json:"duration"`
	Size           string            `json:"size"`
	BitRate        string            `json:"bit_rate"`
	Tags           map[string]string `json:"tags"`
}

// Stream is one elementary stream (`-show_streams`): video, audio, subtitle, or
// data. Fields irrelevant to a given codec type are simply absent / zero.
type Stream struct {
	Index        int    `json:"index"`
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	BitRate      string `json:"bit_rate"`
	Duration     string `json:"duration"`
	AvgFrameRate string `json:"avg_frame_rate"`
	RFrameRate   string `json:"r_frame_rate"`
	Channels     int    `json:"channels"`
	SampleRate   string `json:"sample_rate"`
}

// ErrFFprobeNotFound is returned (wrapped) when the ffprobe binary cannot be
// located on PATH. It surfaces to DuckDB as a clear, actionable error rather
// than a NULL — a missing native dependency is a configuration problem, not
// "this input is not media".
var ErrFFprobeNotFound = errors.New("ffprobe not found on PATH: install ffmpeg (e.g. `brew install ffmpeg`) so the `ffprobe` binary is available")

// ProbePath runs ffprobe against a filesystem path. ffprobe opens the file
// itself (no piping), which is the most capable path: it can seek, so formats
// needing a moov atom at the end of the file (some MP4s) probe correctly.
func ProbePath(ctx context.Context, path string) (*ProbeResult, error) {
	return runFFprobe(ctx, path, nil)
}

// ProbeBytes runs ffprobe against in-memory media by piping the bytes to the
// child's stdin (`-i pipe:0`). Used when the SQL input is a BLOB rather than a
// path. Piping is non-seekable, so a few container layouts that ffprobe can
// only parse by seeking may probe less completely than ProbePath — that is an
// inherent ffprobe/stdin limitation, documented in README/CLAUDE.
func ProbeBytes(ctx context.Context, data []byte) (*ProbeResult, error) {
	return runFFprobe(ctx, "pipe:0", data)
}

// runFFprobe is the single subprocess seam. It always runs ffprobe in a
// child process with -v error (so only real errors hit stderr) and a bounded
// timeout. A non-zero exit (the normal outcome for garbage / truncated / empty
// input) returns a non-nil error; callers translate that into NULL / no rows.
//
// Distinguishes "ffprobe binary missing" (ErrFFprobeNotFound, an actionable
// config error) from "ffprobe ran but rejected the input" (a plain error the
// caller swallows to NULL).
func runFFprobe(ctx context.Context, input string, stdin []byte) (*ProbeResult, error) {
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	args := []string{
		"-v", "error",
		"-hide_banner",
		"-analyzeduration", "10M",
		"-probesize", maxProbeBytes,
		"-print_format", "json",
		"-show_format",
		"-show_streams",
	}
	if stdin != nil {
		args = append(args, "-i", input)
	} else {
		args = append(args, input)
	}

	cmd := exec.CommandContext(cctx, ffprobeBinary, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Binary truly absent → actionable error, not a NULL. This covers both
		// PATH-lookup failures (*exec.Error wrapping exec.ErrNotFound) and an
		// explicit path that does not exist (*fs.PathError from fork/exec).
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
			return nil, ErrFFprobeNotFound
		}
		var execErr *exec.Error
		if errors.As(err, &execErr) {
			return nil, ErrFFprobeNotFound
		}
		// Otherwise ffprobe ran and rejected the input (non-media, truncated,
		// timed out, …). Surface a plain error; the caller decides whether to
		// swallow it to NULL.
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New("ffprobe: " + msg)
	}

	var res ProbeResult
	if err := json.Unmarshal(stdout.Bytes(), &res); err != nil {
		return nil, errors.New("ffprobe: malformed JSON output: " + err.Error())
	}
	return &res, nil
}

// ---------------------------------------------------------------------------
// Accessors. ffprobe emits all numeric values as JSON strings ("1.000000",
// "30/1", …); these helpers parse them, returning (value, ok). ok=false means
// the field is absent or unparseable, which callers map to SQL NULL.
// ---------------------------------------------------------------------------

// FirstVideo returns the first video stream, if any.
func (r *ProbeResult) FirstVideo() (*Stream, bool) { return r.firstOfType("video") }

// FirstAudio returns the first audio stream, if any.
func (r *ProbeResult) FirstAudio() (*Stream, bool) { return r.firstOfType("audio") }

func (r *ProbeResult) firstOfType(t string) (*Stream, bool) {
	for i := range r.Streams {
		if r.Streams[i].CodecType == t {
			return &r.Streams[i], true
		}
	}
	return nil, false
}

// Duration returns the container duration in seconds.
func (r *ProbeResult) Duration() (float64, bool) { return parseFloat(r.Format.Duration) }

// BitRate returns the container bit rate in bits/sec.
func (r *ProbeResult) BitRate() (int64, bool) { return parseInt(r.Format.BitRate) }

// Size returns the container size in bytes.
func (r *ProbeResult) Size() (int64, bool) { return parseInt(r.Format.Size) }

// FPS computes frames-per-second from a stream's avg_frame_rate ("30/1").
func (s *Stream) FPS() (float64, bool) { return parseRate(s.AvgFrameRate) }

// BitRateInt parses a stream's bit_rate.
func (s *Stream) BitRateInt() (int64, bool) { return parseInt(s.BitRate) }

// DurationFloat parses a stream's duration.
func (s *Stream) DurationFloat() (float64, bool) { return parseFloat(s.Duration) }

// SampleRateInt parses an audio stream's sample_rate.
func (s *Stream) SampleRateInt() (int64, bool) { return parseInt(s.SampleRate) }

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func parseInt(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0, false
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// parseRate parses an ffmpeg fraction like "30/1" or "30000/1001" into a float.
// "0/0" (used by ffmpeg for "unknown") returns ok=false.
func parseRate(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "N/A" {
		return 0, false
	}
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return parseFloat(s)
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0, false
	}
	return n / d, true
}
