package config

import "github.com/urfave/cli/v3"

// Gemini holds Gemini LLM configuration
type Gemini struct {
	ProjectID string
	Location  string
	Model     string
}

// Flags returns CLI flags for Gemini configuration
func (c *Gemini) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "gemini-project-id",
			Usage:       "Google Cloud Project ID for Gemini",
			Required:    true,
			Destination: &c.ProjectID,
			Sources:     cli.EnvVars("SHEPHERD_GEMINI_PROJECT_ID"),
		},
		&cli.StringFlag{
			Name:        "gemini-location",
			Usage:       "Vertex AI location/region",
			Value:       "us-central1",
			Destination: &c.Location,
			Sources:     cli.EnvVars("SHEPHERD_GEMINI_LOCATION"),
		},
		&cli.StringFlag{
			Name:        "gemini-model",
			Usage:       "Gemini model to use",
			Value:       "gemini-2.5-flash",
			Destination: &c.Model,
			Sources:     cli.EnvVars("SHEPHERD_GEMINI_MODEL"),
		},
	}
}
