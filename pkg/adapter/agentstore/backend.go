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
	Put(ctx context.Context, key string) (io.WriteCloser, error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Close() error
}

const (
	historyPrefix = "history/v1/"
	tracePrefix   = "trace/v1/"
)
