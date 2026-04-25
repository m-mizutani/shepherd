package config

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/adapter/agentstore"
	"github.com/urfave/cli/v3"
)

// AgentStorage configures the persistent backend used for gollem agent
// history (per Slack ticket conversation) and execution traces.
//
// Exactly one of the following groups must be configured:
//
//   - --agent-storage-fs-dir
//   - --agent-storage-gcs-bucket (optionally with --agent-storage-gcs-prefix)
//
// Both at once is an error; neither is also an error — agent persistence is
// not optional, since the agent's "remember the previous turn" behaviour
// depends on it.
type AgentStorage struct {
	fsDir     string
	gcsBucket string
	gcsPrefix string
}

const defaultGCSPrefix = "shepherd/"

// Flags returns the urfave/cli flags for agent storage configuration.
func (x *AgentStorage) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "agent-storage-fs-dir",
			Usage:       "Local filesystem directory for agent history & trace storage (mutually exclusive with --agent-storage-gcs-bucket)",
			Category:    "Agent Storage",
			Sources:     cli.EnvVars("SHEPHERD_AGENT_STORAGE_FS_DIR"),
			Destination: &x.fsDir,
		},
		&cli.StringFlag{
			Name:        "agent-storage-gcs-bucket",
			Usage:       "GCS bucket for agent history & trace storage (mutually exclusive with --agent-storage-fs-dir)",
			Category:    "Agent Storage",
			Sources:     cli.EnvVars("SHEPHERD_AGENT_STORAGE_GCS_BUCKET"),
			Destination: &x.gcsBucket,
		},
		&cli.StringFlag{
			Name:        "agent-storage-gcs-prefix",
			Usage:       "Object name prefix under --agent-storage-gcs-bucket",
			Category:    "Agent Storage",
			Sources:     cli.EnvVars("SHEPHERD_AGENT_STORAGE_GCS_PREFIX"),
			Value:       defaultGCSPrefix,
			Destination: &x.gcsPrefix,
		},
	}
}

// LogValue returns a structured representation safe to emit in logs.
func (x *AgentStorage) LogValue() slog.Value {
	switch {
	case x.fsDir != "":
		return slog.GroupValue(
			slog.String("backend", "fs"),
			slog.String("dir", x.fsDir),
		)
	case x.gcsBucket != "":
		return slog.GroupValue(
			slog.String("backend", "gcs"),
			slog.String("bucket", x.gcsBucket),
			slog.String("prefix", x.gcsPrefix),
		)
	default:
		return slog.GroupValue(slog.String("backend", "none"))
	}
}

// Configure builds the history & trace repositories. Returns an error if the
// flag combination is invalid or if backend construction fails. Callers must
// invoke Close on the returned io.Closer to release resources (only meaningful
// for the GCS backend).
func (x *AgentStorage) Configure(ctx context.Context) (*agentstore.HistoryRepository, *agentstore.TraceRepository, agentstore.Backend, error) {
	hasFS := x.fsDir != ""
	hasGCS := x.gcsBucket != ""

	switch {
	case hasFS && hasGCS:
		return nil, nil, nil, goerr.New("--agent-storage-fs-dir and --agent-storage-gcs-bucket are mutually exclusive")
	case !hasFS && !hasGCS:
		return nil, nil, nil, goerr.New("agent storage is required: set either --agent-storage-fs-dir or --agent-storage-gcs-bucket")
	}

	var (
		backend agentstore.Backend
		err     error
	)
	if hasFS {
		backend, err = agentstore.NewFileBackend(x.fsDir)
		if err != nil {
			return nil, nil, nil, goerr.Wrap(err, "failed to construct file backend")
		}
	} else {
		backend, err = agentstore.NewGCSBackend(ctx, x.gcsBucket, x.gcsPrefix)
		if err != nil {
			return nil, nil, nil, goerr.Wrap(err, "failed to construct GCS backend")
		}
	}

	return agentstore.NewHistoryRepository(backend), agentstore.NewTraceRepository(backend), backend, nil
}
