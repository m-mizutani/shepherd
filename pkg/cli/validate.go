package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

func cmdValidate() *cli.Command {
	var workspaceCfg config.WorkspaceFiles

	return &cli.Command{
		Name:  "validate",
		Usage: "Validate configuration files",
		Flags: workspaceCfg.Flags(),
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := logging.Default()

			workspaceConfigs, err := workspaceCfg.Configure()
			if err != nil {
				return err
			}

			for _, cfg := range workspaceConfigs {
				logger.Info("Workspace validated",
					slog.String("id", cfg.ID),
					slog.String("name", cfg.Name),
					slog.Int("statuses", len(cfg.FieldSchema.Statuses)),
					slog.Int("fields", len(cfg.FieldSchema.Fields)),
				)
			}

			fmt.Printf("All %d workspace(s) validated successfully.\n", len(workspaceConfigs))
			return nil
		},
	}
}
