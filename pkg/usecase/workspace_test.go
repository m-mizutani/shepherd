package usecase_test

import (
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func setupWorkspaceUseCase(t *testing.T) *usecase.WorkspaceUseCase {
	t.Helper()
	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace:      model.Workspace{ID: "alpha", Name: "Alpha"},
		FieldSchema:    &config.FieldSchema{Statuses: []config.StatusDef{{ID: "open", Name: "Open"}}},
		SlackChannelID: "C1",
	})
	registry.Register(&model.WorkspaceEntry{
		Workspace:      model.Workspace{ID: "beta", Name: "Beta"},
		FieldSchema:    &config.FieldSchema{Statuses: []config.StatusDef{{ID: "new", Name: "New"}}},
		SlackChannelID: "C2",
	})
	return usecase.NewWorkspaceUseCase(registry)
}

func TestWorkspaceUseCase_List(t *testing.T) {
	uc := setupWorkspaceUseCase(t)
	workspaces := uc.List()
	gt.A(t, workspaces).Length(2)
	gt.S(t, string(workspaces[0].ID)).Equal("alpha")
	gt.S(t, string(workspaces[1].ID)).Equal("beta")
}

func TestWorkspaceUseCase_Get(t *testing.T) {
	uc := setupWorkspaceUseCase(t)
	ws := gt.R1(uc.Get(types.WorkspaceID("alpha"))).NoError(t)
	gt.S(t, ws.Name).Equal("Alpha")
}

func TestWorkspaceUseCase_Get_NotFound(t *testing.T) {
	uc := setupWorkspaceUseCase(t)
	_, err := uc.Get(types.WorkspaceID("nonexistent"))
	gt.Error(t, err)
}

func TestWorkspaceUseCase_GetConfig(t *testing.T) {
	uc := setupWorkspaceUseCase(t)
	schema := gt.R1(uc.GetConfig(types.WorkspaceID("alpha"))).NoError(t)
	gt.A(t, schema.Statuses).Length(1)
	gt.S(t, string(schema.Statuses[0].ID)).Equal("open")
}

func TestWorkspaceUseCase_GetConfig_NotFound(t *testing.T) {
	uc := setupWorkspaceUseCase(t)
	_, err := uc.GetConfig(types.WorkspaceID("nonexistent"))
	gt.Error(t, err)
}
