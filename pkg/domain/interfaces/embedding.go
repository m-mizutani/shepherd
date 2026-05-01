package interfaces

import (
	"context"

	"cloud.google.com/go/firestore"
)

// Embedder turns a free-form text into an embedding vector. Implementations
// own the choice of model, dimension, and provider; consumers see a single
// Generate call and treat the returned vector as opaque.
//
// The returned modelID is a stable identifier of the model that produced the
// vector (e.g. "gemini:gemini-embedding-2"). Persisted alongside the vector
// so callers can detect "embedded with a different model" later and force
// a re-embed without comparing the vectors themselves.
type Embedder interface {
	Generate(ctx context.Context, text string) (vec firestore.Vector32, modelID string, err error)
}
