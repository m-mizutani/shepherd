package config

import "github.com/urfave/cli/v3"

// Server holds server configuration
type Server struct {
	Addr string
}

// Flags returns CLI flags for server configuration
func (c *Server) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "addr",
			Usage:       "Server address",
			Value:       "localhost:8080",
			Destination: &c.Addr,
			Sources:     cli.EnvVars("SHEPHERD_ADDR"),
		},
	}
}
