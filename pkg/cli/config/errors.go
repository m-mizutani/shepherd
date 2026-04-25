package config

import (
	"github.com/m-mizutani/goerr/v2"
)

var (
	ErrConfigNotFound       = goerr.New("config file not found")
	ErrNoConfigFiles        = goerr.New("no config files found")
	ErrMissingWorkspaceID   = goerr.New("missing workspace ID")
	ErrInvalidWorkspaceID   = goerr.New("invalid workspace ID")
	ErrDuplicateWorkspaceID = goerr.New("duplicate workspace ID")
	ErrMissingChannelID     = goerr.New("slack channel is required")
	ErrDuplicateChannelID   = goerr.New("duplicate slack channel across workspaces")
)

const (
	ConfigPathKey  = "config_path"
	WorkspaceIDKey = "workspace_id"
)
