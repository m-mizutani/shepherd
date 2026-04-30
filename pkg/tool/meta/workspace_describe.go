package meta

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

type workspaceDescribeTool struct {
	registry *model.WorkspaceRegistry
}

func newWorkspaceDescribeTool(r *model.WorkspaceRegistry) gollem.Tool {
	return &workspaceDescribeTool{registry: r}
}

func (t *workspaceDescribeTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "workspace_describe",
		Description: "Describe the active workspace's status definitions and custom field schema. " +
			"No arguments. " +
			"Returns `{ workspace_id, workspace_name, slack_channel_id, default_status_id, statuses: [{id, name, color, order, closed}], fields: [{id, name, type, required, description, options: [{id, name}]}] }`. " +
			"Use this to translate raw status IDs returned by other tools into names, or to discover what custom fields tickets carry.",
		Parameters:  map[string]*gollem.Parameter{},
	}
}

func (t *workspaceDescribeTool) Run(ctx context.Context, _ map[string]any) (map[string]any, error) {
	wsID, ok := types.WorkspaceFromContext(ctx)
	if !ok || wsID == "" {
		return nil, goerr.New("no active workspace bound to context")
	}
	if t.registry == nil {
		return nil, goerr.New("workspace registry is unavailable")
	}
	entry, ok := t.registry.Get(wsID)
	if !ok {
		return nil, goerr.New("workspace not registered", goerr.V("workspace_id", string(wsID)))
	}

	statuses := make([]map[string]any, 0, len(entry.FieldSchema.Statuses))
	for _, s := range entry.FieldSchema.Statuses {
		statuses = append(statuses, map[string]any{
			"id":     string(s.ID),
			"name":   s.Name,
			"color":  s.Color,
			"order":  s.Order,
			"closed": entry.FieldSchema.IsClosedStatus(s.ID),
		})
	}

	fields := make([]map[string]any, 0, len(entry.FieldSchema.Fields))
	for _, f := range entry.FieldSchema.Fields {
		opts := make([]map[string]any, 0, len(f.Options))
		for _, o := range f.Options {
			opts = append(opts, map[string]any{
				"id":   o.ID,
				"name": o.Name,
			})
		}
		fields = append(fields, map[string]any{
			"id":          f.ID,
			"name":        f.Name,
			"type":        string(f.Type),
			"required":    f.Required,
			"description": f.Description,
			"options":     opts,
		})
	}

	return map[string]any{
		"workspace_id":      string(entry.Workspace.ID),
		"workspace_name":    entry.Workspace.Name,
		"slack_channel_id":  string(entry.SlackChannelID),
		"default_status_id": string(entry.FieldSchema.TicketConfig.DefaultStatusID),
		"statuses":          statuses,
		"fields":            fields,
	}, nil
}
