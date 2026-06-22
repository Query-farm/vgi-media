// Copyright 2026 Query Farm LLC - https://query.farm

package mediaworker

import (
	"context"

	"github.com/apache/arrow-go/v18/arrow/array"
)

// CatalogName is the VGI catalog name advertised by this worker.
const CatalogName = "media"

// probeRow probes a single input row from an `arrow_type="any"` column. The
// input is either a VARCHAR path (String / Dictionary-of-String) or a BLOB
// (Binary / LargeBinary):
//
//   - VARCHAR  → treated as a filesystem path; ffprobe opens it directly.
//   - BLOB     → bytes are piped to ffprobe's stdin (-i pipe:0).
//
// Returns (nil, false) for a NULL row or an unsupported column type, so the
// scalar/table callers map it to SQL NULL / no rows. An ffprobe failure on
// non-media / garbage input also yields (nil, false) — the worker swallows the
// error rather than propagating it, so untrusted bytes can never crash a query.
// The single exception is a MISSING ffprobe binary, which is an actionable
// configuration error and is returned to the caller.
func probeRow(ctx context.Context, col interface{ IsNull(int) bool }, i int) (*ProbeResult, error) {
	switch c := col.(type) {
	case *array.String:
		if c.IsNull(i) {
			return nil, nil
		}
		return swallow(ProbePath(ctx, c.Value(i)))
	case *array.LargeString:
		if c.IsNull(i) {
			return nil, nil
		}
		return swallow(ProbePath(ctx, c.Value(i)))
	case *array.Binary:
		if c.IsNull(i) {
			return nil, nil
		}
		return swallow(ProbeBytes(ctx, c.Value(i)))
	case *array.LargeBinary:
		if c.IsNull(i) {
			return nil, nil
		}
		return swallow(ProbeBytes(ctx, c.Value(i)))
	default:
		// Unsupported column type (e.g. an integer) → NULL/no rows.
		return nil, nil
	}
}

// swallow converts a "ffprobe rejected this input" error into (nil, nil) so the
// caller emits NULL / no rows, but propagates ErrFFprobeNotFound (a missing
// native dependency) so DuckDB shows a clear, actionable message.
func swallow(res *ProbeResult, err error) (*ProbeResult, error) {
	if err != nil {
		if err == ErrFFprobeNotFound {
			return nil, err
		}
		return nil, nil
	}
	return res, nil
}
