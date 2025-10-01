package config

import (
	"log/slog"
	"os"

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
	switch c.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
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
