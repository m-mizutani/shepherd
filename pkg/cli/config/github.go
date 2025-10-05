package config

import "github.com/urfave/cli/v3"

// GitHub holds GitHub configuration
type GitHub struct {
	WebhookSecret  string
	AppID          int64
	InstallationID int64
	PrivateKey     string
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
		&cli.Int64Flag{
			Name:        "github-app-id",
			Usage:       "GitHub App ID",
			Required:    true,
			Destination: &c.AppID,
			Sources:     cli.EnvVars("SHEPHERD_GITHUB_APP_ID"),
		},
		&cli.Int64Flag{
			Name:        "github-installation-id",
			Usage:       "GitHub App Installation ID",
			Required:    true,
			Destination: &c.InstallationID,
			Sources:     cli.EnvVars("SHEPHERD_GITHUB_INSTALLATION_ID"),
		},
		&cli.StringFlag{
			Name:        "github-private-key",
			Usage:       "GitHub App private key (PEM format)",
			Required:    true,
			Destination: &c.PrivateKey,
			Sources:     cli.EnvVars("SHEPHERD_GITHUB_PRIVATE_KEY"),
		},
	}
}
