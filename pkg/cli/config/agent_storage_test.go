package config_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/urfave/cli/v3"
)

func runAgentStorage(t *testing.T, args []string) (*config.AgentStorage, error) {
	t.Helper()
	for _, k := range []string{
		"SHEPHERD_AGENT_STORAGE_FS_DIR",
		"SHEPHERD_AGENT_STORAGE_GCS_BUCKET",
		"SHEPHERD_AGENT_STORAGE_GCS_PREFIX",
	} {
		t.Setenv(k, "")
	}
	var cfg config.AgentStorage
	app := &cli.Command{
		Flags: cfg.Flags(),
		Action: func(_ context.Context, _ *cli.Command) error {
			return nil
		},
	}
	err := app.Run(context.Background(), append([]string{"app"}, args...))
	return &cfg, err
}

func TestAgentStorage_FileBackend(t *testing.T) {
	dir := t.TempDir()
	cfg, err := runAgentStorage(t, []string{"--agent-storage-fs-dir", dir})
	gt.NoError(t, err)

	hist, tr, be, err := cfg.Configure(context.Background())
	gt.NoError(t, err)
	gt.NotNil(t, hist)
	gt.NotNil(t, tr)
	gt.NotNil(t, be)
	t.Cleanup(func() { _ = be.Close() })
}

func TestAgentStorage_BothBackendsIsError(t *testing.T) {
	cfg, err := runAgentStorage(t, []string{
		"--agent-storage-fs-dir", t.TempDir(),
		"--agent-storage-gcs-bucket", "demo",
	})
	gt.NoError(t, err)

	_, _, _, configureErr := cfg.Configure(context.Background())
	gt.Error(t, configureErr)
}

func TestAgentStorage_NeitherBackendIsError(t *testing.T) {
	cfg, err := runAgentStorage(t, nil)
	gt.NoError(t, err)

	_, _, _, configureErr := cfg.Configure(context.Background())
	gt.Error(t, configureErr)
}
