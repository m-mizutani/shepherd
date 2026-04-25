package meta_test

import (
	"context"
	"testing"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/tool/meta"
)

func toolMap(t *testing.T, deps meta.Deps) map[string]gollem.Tool {
	t.Helper()
	out := map[string]gollem.Tool{}
	for _, tool := range meta.Tools(deps) {
		out[tool.Spec().Name] = tool
	}
	return out
}

func TestCurrentTime(t *testing.T) {
	fixed := time.Date(2026, 4, 25, 9, 0, 0, 0, time.UTC)
	tools := toolMap(t, meta.Deps{Now: func() time.Time { return fixed }})
	out, err := tools["current_time"].Run(context.Background(), nil)
	gt.NoError(t, err)
	gt.Equal(t, out["rfc3339"], "2026-04-25T09:00:00Z")
	gt.Equal(t, out["unix"].(int64), fixed.Unix())
}

func TestWorkspaceDescribe(t *testing.T) {
	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: "ws-1", Name: "Sec"},
		FieldSchema: &config.FieldSchema{
			Statuses: []config.StatusDef{
				{ID: "open", Name: "Open"},
				{ID: "closed", Name: "Closed"},
			},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
				ClosedStatusIDs: []types.StatusID{"closed"},
			},
			Fields: []config.FieldDefinition{
				{ID: "severity", Name: "Severity", Type: types.FieldTypeText},
			},
		},
		SlackChannelID: "C-1",
	})
	tools := toolMap(t, meta.Deps{Registry: registry})

	t.Run("happy path", func(t *testing.T) {
		ctx := types.ContextWithWorkspace(context.Background(), "ws-1")
		out, err := tools["workspace_describe"].Run(ctx, nil)
		gt.NoError(t, err)
		gt.Equal(t, out["workspace_name"], "Sec")
		gt.Equal(t, out["default_status_id"], "open")
		statuses := out["statuses"].([]map[string]any)
		gt.Equal(t, len(statuses), 2)
		gt.Equal(t, statuses[1]["closed"].(bool), true)
		fields := out["fields"].([]map[string]any)
		gt.Equal(t, fields[0]["id"], "severity")
	})

	t.Run("missing context errors", func(t *testing.T) {
		_, err := tools["workspace_describe"].Run(context.Background(), nil)
		gt.Error(t, err)
	})

	t.Run("unregistered workspace errors", func(t *testing.T) {
		ctx := types.ContextWithWorkspace(context.Background(), "ws-missing")
		_, err := tools["workspace_describe"].Run(ctx, nil)
		gt.Error(t, err)
	})
}
