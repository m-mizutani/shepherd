package config

import (
	"log/slog"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/urfave/cli/v3"
)

type Sentry struct {
	dsn string
	env string
}

func (x *Sentry) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "sentry-dsn",
			Usage:       "Sentry DSN",
			Sources:     cli.EnvVars("SHEPHERD_SENTRY_DSN"),
			Destination: &x.dsn,
		},
		&cli.StringFlag{
			Name:        "sentry-env",
			Usage:       "Sentry environment",
			Value:       "development",
			Sources:     cli.EnvVars("SHEPHERD_SENTRY_ENV"),
			Destination: &x.env,
		},
	}
}

func (x *Sentry) Configure() (func(), error) {
	if x.dsn == "" {
		return func() {}, nil
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         x.dsn,
		Environment: x.env,
	}); err != nil {
		return nil, goerr.Wrap(err, "failed to initialize Sentry")
	}

	logging.Default().Info("Sentry initialized", slog.String("env", x.env))
	return func() { sentry.Flush(2 * time.Second) }, nil
}
