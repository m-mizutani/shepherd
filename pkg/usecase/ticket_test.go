package usecase_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func setupTicketUseCase(t *testing.T) (*usecase.TicketUseCase, *model.WorkspaceRegistry) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: "ws-test", Name: "Test"},
		FieldSchema: &config.FieldSchema{
			Statuses: []config.StatusDef{
				{ID: "open", Name: "Open"},
				{ID: "in-progress", Name: "In Progress"},
				{ID: "resolved", Name: "Resolved"},
				{ID: "closed", Name: "Closed"},
			},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
				ClosedStatusIDs: []string{"resolved", "closed"},
			},
		},
		SlackChannelID: "C111",
	})

	uc := usecase.NewTicketUseCase(repo, registry, nil)
	return uc, registry
}

func TestTicketUseCase_Create(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket, err := uc.Create(ctx, "ws-test", "My Ticket", "desc", "", "", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ticket.Title != "My Ticket" {
		t.Errorf("expected title 'My Ticket', got %q", ticket.Title)
	}
	if ticket.StatusID != "open" {
		t.Errorf("expected default status 'open', got %q", ticket.StatusID)
	}
	if ticket.ID == "" {
		t.Error("expected non-empty ticket ID")
	}
}

func TestTicketUseCase_Create_WithStatus(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket, err := uc.Create(ctx, "ws-test", "Custom Status", "", "in-progress", "", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ticket.StatusID != "in-progress" {
		t.Errorf("expected status 'in-progress', got %q", ticket.StatusID)
	}
}

func TestTicketUseCase_Create_WithFields(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	fields := map[string]model.FieldValue{
		"priority": {FieldID: "priority", Type: types.FieldTypeSelect, Value: "high"},
	}
	ticket, err := uc.Create(ctx, "ws-test", "With Fields", "", "", "", fields)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if fv, ok := ticket.FieldValues["priority"]; !ok || fv.Value != "high" {
		t.Errorf("expected field priority=high, got %+v", ticket.FieldValues)
	}
}

func TestTicketUseCase_Create_UnknownWorkspace(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	_, err := uc.Create(ctx, "nonexistent", "Title", "", "", "", nil)
	if err == nil {
		t.Fatal("expected error for unknown workspace")
	}
}

func TestTicketUseCase_GetAndList(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created, err := uc.Create(ctx, "ws-test", "T1", "", "", "", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := uc.Get(ctx, "ws-test", created.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Title != "T1" {
		t.Errorf("expected title 'T1', got %q", got.Title)
	}

	tickets, err := uc.List(ctx, "ws-test", nil, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(tickets) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(tickets))
	}
}

func TestTicketUseCase_List_FilterByClosed(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	uc.Create(ctx, "ws-test", "Open Ticket", "", "open", "", nil)
	uc.Create(ctx, "ws-test", "Resolved Ticket", "", "resolved", "", nil)
	uc.Create(ctx, "ws-test", "Closed Ticket", "", "closed", "", nil)

	isClosed := true
	closedTickets, err := uc.List(ctx, "ws-test", &isClosed, nil)
	if err != nil {
		t.Fatalf("List closed failed: %v", err)
	}
	if len(closedTickets) != 2 {
		t.Errorf("expected 2 closed tickets, got %d", len(closedTickets))
	}

	isOpen := false
	openTickets, err := uc.List(ctx, "ws-test", &isOpen, nil)
	if err != nil {
		t.Fatalf("List open failed: %v", err)
	}
	if len(openTickets) != 1 {
		t.Errorf("expected 1 open ticket, got %d", len(openTickets))
	}
}

func TestTicketUseCase_Update(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created, err := uc.Create(ctx, "ws-test", "Original", "desc", "", "", nil)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newTitle := "Updated"
	newStatus := "in-progress"
	updated, err := uc.Update(ctx, "ws-test", created.ID, &newTitle, nil, &newStatus, nil, nil)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", updated.Title)
	}
	if updated.StatusID != "in-progress" {
		t.Errorf("expected status 'in-progress', got %q", updated.StatusID)
	}
	if updated.Description != "desc" {
		t.Errorf("expected description preserved, got %q", updated.Description)
	}
}

func TestTicketUseCase_Update_MergeFields(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	initial := map[string]model.FieldValue{
		"priority": {FieldID: "priority", Value: "high"},
	}
	created, _ := uc.Create(ctx, "ws-test", "T", "", "", "", initial)

	newFields := map[string]model.FieldValue{
		"category": {FieldID: "category", Value: "bug"},
	}
	updated, err := uc.Update(ctx, "ws-test", created.ID, nil, nil, nil, nil, newFields)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if _, ok := updated.FieldValues["priority"]; !ok {
		t.Error("expected existing field 'priority' to be preserved")
	}
	if _, ok := updated.FieldValues["category"]; !ok {
		t.Error("expected new field 'category' to be added")
	}
}

func TestTicketUseCase_Delete(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created, _ := uc.Create(ctx, "ws-test", "To Delete", "", "", "", nil)

	if err := uc.Delete(ctx, "ws-test", created.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := uc.Get(ctx, "ws-test", created.ID)
	if err == nil {
		t.Fatal("expected error getting deleted ticket")
	}
}
