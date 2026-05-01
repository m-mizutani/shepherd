package config

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/service/embedding"
	"github.com/urfave/cli/v3"
)

// Embedding holds CLI configuration for the Gemini-backed text embedding
// service. The chat-side LLM provider (config.LLM) is intentionally
// independent: the embedding lane stays on Gemini regardless of which model
// is chosen for chat / triage so that semantic ticket search keeps working
// when chat runs on Claude or OpenAI.
type Embedding struct {
	projectID string
	location  string
	model     string
	dim       int
}

func (x *Embedding) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "embedding-gemini-project",
			Usage:       "GCP project ID for the Gemini embedding endpoint (required to enable ticket embeddings)",
			Sources:     cli.EnvVars("SHEPHERD_EMBEDDING_GEMINI_PROJECT"),
			Destination: &x.projectID,
		},
		&cli.StringFlag{
			Name:        "embedding-gemini-location",
			Usage:       "Vertex AI location for embeddings: one of global, us, eu (default global)",
			Sources:     cli.EnvVars("SHEPHERD_EMBEDDING_GEMINI_LOCATION"),
			Value:       "global",
			Destination: &x.location,
		},
		&cli.StringFlag{
			Name:        "embedding-gemini-model",
			Usage:       "Gemini embedding model name",
			Sources:     cli.EnvVars("SHEPHERD_EMBEDDING_GEMINI_MODEL"),
			Value:       "gemini-embedding-2",
			Destination: &x.model,
		},
		&cli.IntFlag{
			Name:        "embedding-dim",
			Usage:       "Embedding output dimension. Must match the Firestore vector index. Recommended: 768 or 1536",
			Sources:     cli.EnvVars("SHEPHERD_EMBEDDING_DIM"),
			Value:       768,
			Destination: &x.dim,
		},
	}
}

// IsEnabled reports whether the embedding service has been configured.
func (x *Embedding) IsEnabled() bool { return x.projectID != "" }

// ProjectID exposes the configured project for logging.
func (x *Embedding) ProjectID() string { return x.projectID }

// Location exposes the configured location for logging.
func (x *Embedding) Location() string { return x.location }

// Model exposes the configured model name for logging.
func (x *Embedding) Model() string { return x.model }

// Dim exposes the configured embedding dimension for logging.
func (x *Embedding) Dim() int { return x.dim }

// NewService constructs the embedding service. The constructor performs a
// live self-test against the API; any failure (auth, network, model name,
// dimension rejection) is returned as an error so the caller can refuse to
// start the server.
func (x *Embedding) NewService(ctx context.Context) (*embedding.Service, error) {
	if !x.IsEnabled() {
		return nil, goerr.New("embedding-gemini-project is required")
	}
	svc, err := embedding.New(ctx, x.projectID, x.location, x.model, x.dim)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to initialise embedding service",
			goerr.V("project", x.projectID),
			goerr.V("location", x.location),
			goerr.V("model", x.model),
			goerr.V("dim", x.dim),
		)
	}
	return svc, nil
}
