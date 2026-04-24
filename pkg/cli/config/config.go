package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
	"github.com/pelletier/go-toml/v2"
	"github.com/urfave/cli/v3"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type WorkspaceFiles struct {
	paths []string
}

func (x *WorkspaceFiles) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{
			Name:        "config",
			Usage:       "Workspace config file or directory path (can be specified multiple times)",
			Sources:     cli.EnvVars("SHEPHERD_CONFIG"),
			Value:       []string{"./config.toml"},
			Destination: &x.paths,
		},
	}
}

func (x *WorkspaceFiles) Configure() ([]*WorkspaceConfig, error) {
	return LoadWorkspaceConfigs(x.paths)
}

type WorkspaceBaseConfig struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
}

type TicketSection struct {
	DefaultStatus  string   `toml:"default_status"`
	ClosedStatuses []string `toml:"closed_statuses"`
}

type SlackSection struct {
	ChannelID string `toml:"channel_id"`
}

type StatusConfig struct {
	ID    string `toml:"id"`
	Name  string `toml:"name"`
	Color string `toml:"color"`
}

type FieldOptionConfig struct {
	ID       string         `toml:"id"`
	Name     string         `toml:"name"`
	Color    string         `toml:"color"`
	Metadata map[string]any `toml:"metadata"`
}

type FieldConfig struct {
	ID          string              `toml:"id"`
	Name        string              `toml:"name"`
	Type        string              `toml:"type"`
	Required    bool                `toml:"required"`
	Description string              `toml:"description"`
	Options     []FieldOptionConfig `toml:"options"`
}

type LabelsConfig struct {
	Ticket      string `toml:"ticket"`
	Title       string `toml:"title"`
	Description string `toml:"description"`
}

type AppConfig struct {
	Workspace WorkspaceBaseConfig `toml:"workspace"`
	Ticket    TicketSection       `toml:"ticket"`
	Slack     SlackSection        `toml:"slack"`
	Statuses  []StatusConfig      `toml:"statuses"`
	Fields    []FieldConfig       `toml:"fields"`
	Labels    LabelsConfig        `toml:"labels"`
}

type WorkspaceConfig struct {
	ID             string
	Name           string
	SlackChannelID string
	FieldSchema    *domainConfig.FieldSchema
}

func (a *AppConfig) Validate() error {
	wsID := a.Workspace.ID
	if wsID == "" {
		return goerr.Wrap(ErrMissingWorkspaceID, "[workspace] id is required")
	}
	if !idPattern.MatchString(wsID) || len(wsID) > 63 {
		return goerr.Wrap(ErrInvalidWorkspaceID,
			"workspace ID must match ^[a-z0-9]+(-[a-z0-9]+)*$ and be at most 63 characters",
			goerr.V(WorkspaceIDKey, wsID))
	}

	if a.Slack.ChannelID == "" {
		return goerr.Wrap(ErrMissingChannelID, "[slack] channel_id is required",
			goerr.V(WorkspaceIDKey, wsID))
	}

	return nil
}

func (a *AppConfig) ToDomainFieldSchema() *domainConfig.FieldSchema {
	statuses := make([]domainConfig.StatusDef, len(a.Statuses))
	for i, s := range a.Statuses {
		statuses[i] = domainConfig.StatusDef{
			ID:    s.ID,
			Name:  s.Name,
			Color: s.Color,
			Order: i,
		}
	}

	fields := make([]domainConfig.FieldDefinition, len(a.Fields))
	for i, f := range a.Fields {
		options := make([]domainConfig.FieldOption, len(f.Options))
		for j, opt := range f.Options {
			options[j] = domainConfig.FieldOption{
				ID:       opt.ID,
				Name:     opt.Name,
				Color:    opt.Color,
				Metadata: opt.Metadata,
			}
		}
		fields[i] = domainConfig.FieldDefinition{
			ID:          f.ID,
			Name:        f.Name,
			Type:        types.FieldType(f.Type),
			Required:    f.Required,
			Description: f.Description,
			Options:     options,
		}
	}

	labels := domainConfig.EntityLabels{
		Ticket:      a.Labels.Ticket,
		Title:       a.Labels.Title,
		Description: a.Labels.Description,
	}
	if labels.Ticket == "" {
		labels.Ticket = "Ticket"
	}
	if labels.Title == "" {
		labels.Title = "Title"
	}
	if labels.Description == "" {
		labels.Description = "Description"
	}

	defaultStatusID := a.Ticket.DefaultStatus
	if defaultStatusID == "" && len(statuses) > 0 {
		defaultStatusID = statuses[0].ID
	}

	return &domainConfig.FieldSchema{
		Statuses: statuses,
		TicketConfig: domainConfig.TicketConfig{
			DefaultStatusID: defaultStatusID,
			ClosedStatusIDs: a.Ticket.ClosedStatuses,
		},
		Fields: fields,
		Labels: labels,
	}
}

func LoadWorkspaceConfigs(paths []string) ([]*WorkspaceConfig, error) {
	var tomlFiles []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to stat config path", goerr.V(ConfigPathKey,p))
		}

		if info.IsDir() {
			err := filepath.WalkDir(p, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if !d.IsDir() && strings.HasSuffix(d.Name(), ".toml") {
					tomlFiles = append(tomlFiles, path)
				}
				return nil
			})
			if err != nil {
				return nil, goerr.Wrap(err, "failed to walk config directory", goerr.V(ConfigPathKey,p))
			}
		} else {
			tomlFiles = append(tomlFiles, p)
		}
	}

	if len(tomlFiles) == 0 {
		return nil, goerr.Wrap(ErrNoConfigFiles, "no .toml files found in specified paths")
	}

	var configs []*WorkspaceConfig
	seenIDs := make(map[string]string)
	seenChannels := make(map[string]string)
	for _, f := range tomlFiles {
		wc, err := loadSingleWorkspaceConfig(f)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to load workspace config", goerr.V(ConfigPathKey,f))
		}

		if existing, ok := seenIDs[wc.ID]; ok {
			return nil, goerr.Wrap(ErrDuplicateWorkspaceID, "duplicate workspace ID",
				goerr.V(WorkspaceIDKey,wc.ID),
				goerr.V("first_file", existing),
				goerr.V("second_file", f))
		}
		seenIDs[wc.ID] = f

		if existing, ok := seenChannels[wc.SlackChannelID]; ok {
			return nil, goerr.Wrap(ErrDuplicateChannelID, "duplicate slack channel_id",
				goerr.V("channel_id", wc.SlackChannelID),
				goerr.V("first_file", existing),
				goerr.V("second_file", f))
		}
		seenChannels[wc.SlackChannelID] = f

		configs = append(configs, wc)
	}

	return configs, nil
}

func loadSingleWorkspaceConfig(path string) (*WorkspaceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read config file", goerr.V(ConfigPathKey,path))
	}

	var appCfg AppConfig
	if err := toml.Unmarshal(data, &appCfg); err != nil {
		return nil, goerr.Wrap(err, "failed to parse TOML config", goerr.V(ConfigPathKey,path))
	}

	if err := appCfg.Validate(); err != nil {
		return nil, goerr.Wrap(err, "config validation failed", goerr.V(ConfigPathKey, path))
	}

	schema := appCfg.ToDomainFieldSchema()
	if err := schema.Validate(); err != nil {
		return nil, goerr.Wrap(err, "field schema validation failed",
			goerr.V(ConfigPathKey, path), goerr.V(WorkspaceIDKey, appCfg.Workspace.ID))
	}

	wsName := appCfg.Workspace.Name
	if wsName == "" {
		wsName = appCfg.Workspace.ID
	}

	return &WorkspaceConfig{
		ID:             appCfg.Workspace.ID,
		Name:           wsName,
		SlackChannelID: appCfg.Slack.ChannelID,
		FieldSchema:    schema,
	}, nil
}

func BuildRegistry(configs []*WorkspaceConfig) *model.WorkspaceRegistry {
	registry := model.NewWorkspaceRegistry()
	for _, wc := range configs {
		registry.Register(&model.WorkspaceEntry{
			Workspace: model.Workspace{
				ID:   wc.ID,
				Name: wc.Name,
			},
			FieldSchema:    wc.FieldSchema,
			SlackChannelID: wc.SlackChannelID,
		})
		logging.Default().Info("Registered workspace", "id", wc.ID, "name", wc.Name)
	}
	return registry
}
