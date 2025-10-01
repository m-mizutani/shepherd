package cli

import (
	"context"
	"log/slog"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/urfave/cli/v3"
)

// Run runs the CLI application
func Run(ctx context.Context, args []string) error {
	var loggerCfg config.Logger
	var logger *slog.Logger

	app := &cli.Command{
		Name:    "shepherd",
		Usage:   "GitHub App webhook handler",
		Version: types.Version,
		Flags:   loggerCfg.Flags(),
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			var err error
			logger, err = loggerCfg.Configure()
			if err != nil {
				return nil, err
			}

			slog.SetDefault(logger)
			ctx = ctxlog.With(ctx, logger)
			return ctx, nil
		},
		Commands: []*cli.Command{
			cmdServe(),
		},
	}

	if err := app.Run(ctx, args); err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Error("CLI execution failed", slog.Any("error", err))
		return err
	}

	return nil
}
