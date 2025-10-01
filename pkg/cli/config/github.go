package config

import "github.com/urfave/cli/v3"

// GitHub holds GitHub configuration
type GitHub struct {
	WebhookSecret string
}

// Flags returns CLI flags for GitHub configuration
func (c *GitHub) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "github-webhook-secret",
			Usage:       "GitHub webhook secret",
			Required:    true,
			Destination: &c.WebhookSecret,
			Sources:     cli.EnvVars("SHEPHERD_GITHUB_WEBHOOK_SECRET"),
		},
	}
}
