package config

import (
	"log/slog"
	"os"

	"github.com/m-mizutani/goerr/v2"
	"github.com/urfave/cli/v3"
)

// Logger holds logger configuration
type Logger struct {
	Level string
	JSON  bool
}

// Flags returns CLI flags for logger configuration
func (c *Logger) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Usage:       "Log level (debug, info, warn, error)",
			Value:       "info",
			Destination: &c.Level,
			Sources:     cli.EnvVars("SHEPHERD_LOG_LEVEL"),
		},
		&cli.BoolFlag{
			Name:        "log-json",
			Usage:       "Output logs in JSON format",
			Value:       false,
			Destination: &c.JSON,
			Sources:     cli.EnvVars("SHEPHERD_LOG_JSON"),
		},
	}
}

// Configure configures and returns a logger
func (c *Logger) Configure() (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Level)); err != nil {
		// Return an error for invalid log levels to alert the user of a configuration mistake
		return nil, goerr.Wrap(err, "invalid log level", goerr.V("level", c.Level))
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	if c.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler), nil
}
