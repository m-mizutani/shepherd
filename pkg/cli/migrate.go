package cli

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

func cmdMigrate() *cli.Command {
	var (
		repoCfg config.Repository
		dryRun  bool
	)

	flags := repoCfg.Flags()
	flags = append(flags, &cli.BoolFlag{
		Name:        "dry-run",
		Usage:       "Show changes without applying",
		Destination: &dryRun,
	})

	return &cli.Command{
		Name:  "migrate",
		Usage: "Create/update Firestore indexes and run data migrations",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			logger := logging.Default()

			if dryRun {
				logger.Info("Dry run mode: showing planned changes")
			}

			logger.Info("Migration completed (no migrations defined yet)")
			return nil
		},
	}
}
