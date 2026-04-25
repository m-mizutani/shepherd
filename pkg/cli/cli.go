package cli

import (
	"context"

	"github.com/m-mizutani/shepherd/pkg/cli/config"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

func Run(ctx context.Context, args []string) error {
	var loggerCfg config.Logger

	app := &cli.Command{
		Name:  "shepherd",
		Usage: "Slack-integrated ticket management system",
		Flags: loggerCfg.Flags(),
		Before: func(ctx context.Context, c *cli.Command) (context.Context, error) {
			closer, err := loggerCfg.Configure()
			if err != nil {
				return ctx, err
			}
			_ = closer
			return ctx, nil
		},
		Commands: []*cli.Command{
			cmdServe(),
			cmdMigrate(),
			cmdValidate(),
		},
	}

	if err := app.Run(ctx, args); err != nil {
		logging.Default().Error("failed to run app", "error", err)
		return err
	}

	return nil
}
