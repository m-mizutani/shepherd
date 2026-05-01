// Package embedding provides a Gemini-backed text embedding service.
//
// The service is intentionally narrow: a single Generate call that turns a
// string into a firestore.Vector32 plus a stable model identifier. The
// constructor performs a synchronous self-test so that misconfiguration
// (wrong project, missing credentials, unreachable region) surfaces at
// startup time rather than the first request after deploy.
package embedding

import (
	"context"
	"fmt"

	"cloud.google.com/go/firestore"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem/llm/gemini"
)

// embeddingClient is the narrow surface of *gemini.Client we depend on.
// It exists so tests can substitute a fake in export_test.go without
// having to construct a real gemini.Client.
type embeddingClient interface {
	GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error)
}

// Service is a Gemini text embedding service.
type Service struct {
	client  embeddingClient
	modelID string
	dim     int
}

// New constructs a Service backed by Vertex AI Gemini and runs a one-shot
// self-test against the live API. Any failure (auth, network, model name,
// dimension rejection) returns an error so the caller can refuse to start
// the server.
func New(ctx context.Context, projectID, location, model string, dim int) (*Service, error) {
	if projectID == "" {
		return nil, goerr.New("embedding projectID is required")
	}
	if location == "" {
		return nil, goerr.New("embedding location is required")
	}
	if model == "" {
		return nil, goerr.New("embedding model is required")
	}
	if dim <= 0 {
		return nil, goerr.New("embedding dimension must be positive", goerr.V("dim", dim))
	}

	client, err := gemini.New(ctx, projectID, location, gemini.WithEmbeddingModel(model))
	if err != nil {
		return nil, goerr.Wrap(err, "create gemini client",
			goerr.V("project_id", projectID),
			goerr.V("location", location),
			goerr.V("model", model),
		)
	}

	s := &Service{
		client:  client,
		modelID: "gemini:" + model,
		dim:     dim,
	}

	if err := s.selfTest(ctx); err != nil {
		return nil, goerr.Wrap(err, "embedding self-test failed",
			goerr.V("project_id", projectID),
			goerr.V("location", location),
			goerr.V("model", model),
			goerr.V("dim", dim),
		)
	}
	return s, nil
}

// Generate returns the embedding vector for text along with the model id.
func (s *Service) Generate(ctx context.Context, text string) (firestore.Vector32, string, error) {
	if s == nil {
		return nil, "", goerr.New("embedding service is not initialised")
	}
	if text == "" {
		return nil, "", goerr.New("embedding input is empty")
	}

	out, err := s.client.GenerateEmbedding(ctx, s.dim, []string{text})
	if err != nil {
		return nil, "", goerr.Wrap(err, "generate embedding")
	}
	if len(out) != 1 {
		return nil, "", goerr.New("unexpected embedding batch size",
			goerr.V("got", len(out)),
			goerr.V("want", 1),
		)
	}
	if got := len(out[0]); got != s.dim {
		return nil, "", goerr.New("embedding dimension mismatch",
			goerr.V("got", got),
			goerr.V("want", s.dim),
		)
	}

	vec := make(firestore.Vector32, s.dim)
	for i, v := range out[0] {
		vec[i] = float32(v)
	}
	return vec, s.modelID, nil
}

// ModelID returns the stable identifier persisted alongside vectors.
func (s *Service) ModelID() string { return s.modelID }

// Dim returns the configured embedding dimension.
func (s *Service) Dim() int { return s.dim }

func (s *Service) selfTest(ctx context.Context) error {
	const probe = "shepherd"
	_, _, err := s.Generate(ctx, probe)
	if err != nil {
		return fmt.Errorf("probe embedding %q: %w", probe, err)
	}
	return nil
}
