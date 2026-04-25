// Package agentstore provides storage adapters for gollem agent state:
// conversation history (per Slack ticket) and execution traces. Two backends
// are supported, mutually exclusive at configuration time:
//
//   - file: a local filesystem directory.
//   - gcs:  a Google Cloud Storage bucket with object-name prefix.
//
// History and trace data live under separate sub-trees of the same backend
// (history/v1/... and trace/v1/...).
package agentstore

import (
	"context"
	"io"
)

// Backend is the minimal contract every storage backend must satisfy.
//
// Get must return (nil, nil) when the requested key does not exist; any other
// failure must be returned as an error.
type Backend interface {
	Put(ctx context.Context, key string) (Writer, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Close() error
}

// Writer is the per-Put writer returned by Backend.Put. Callers that finish
// writing a complete payload must call Close to commit. Callers that detect
// a mid-stream failure (e.g., JSON encoding error) must call Abort so the
// backend can discard the partial payload — for GCS this avoids finalising a
// truncated upload, and for the filesystem it removes the half-written file.
//
// Calling Close after Abort, or Abort after Close, is a no-op.
type Writer interface {
	io.WriteCloser
	Abort(cause error)
}

const (
	historyPrefix = "history/v1/"
	tracePrefix   = "trace/v1/"
)
