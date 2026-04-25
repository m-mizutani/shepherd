package usecase_test

import (
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
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
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].ID != "alpha" {
		t.Errorf("expected first workspace 'alpha', got %q", workspaces[0].ID)
	}
	if workspaces[1].ID != "beta" {
		t.Errorf("expected second workspace 'beta', got %q", workspaces[1].ID)
	}
}

func TestWorkspaceUseCase_Get(t *testing.T) {
	uc := setupWorkspaceUseCase(t)

	ws, err := uc.Get("alpha")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if ws.Name != "Alpha" {
		t.Errorf("expected name 'Alpha', got %q", ws.Name)
	}
}

func TestWorkspaceUseCase_Get_NotFound(t *testing.T) {
	uc := setupWorkspaceUseCase(t)

	_, err := uc.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent workspace")
	}
}

func TestWorkspaceUseCase_GetConfig(t *testing.T) {
	uc := setupWorkspaceUseCase(t)

	schema, err := uc.GetConfig("alpha")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if len(schema.Statuses) != 1 || schema.Statuses[0].ID != "open" {
		t.Errorf("unexpected schema: %+v", schema)
	}
}

func TestWorkspaceUseCase_GetConfig_NotFound(t *testing.T) {
	uc := setupWorkspaceUseCase(t)

	_, err := uc.GetConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent workspace config")
	}
}
