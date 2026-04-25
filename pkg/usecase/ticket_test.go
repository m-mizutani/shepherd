package usecase_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gt"
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
				ClosedStatusIDs: []types.StatusID{"resolved", "closed"},
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

	ticket := gt.R1(uc.Create(ctx, "ws-test", "My Ticket", "desc", "", "", nil)).NoError(t)
	gt.S(t, ticket.Title).Equal("My Ticket")
	gt.S(t, string(ticket.StatusID)).Equal("open")
	gt.S(t, string(ticket.ID)).NotEqual("")
}

func TestTicketUseCase_Create_WithStatus(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "Custom Status", "", "in-progress", "", nil)).NoError(t)
	gt.S(t, string(ticket.StatusID)).Equal("in-progress")
}

func TestTicketUseCase_Create_WithFields(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	fields := map[string]model.FieldValue{
		"priority": {FieldID: "priority", Type: types.FieldTypeSelect, Value: "high"},
	}
	ticket := gt.R1(uc.Create(ctx, "ws-test", "With Fields", "", "", "", fields)).NoError(t)
	gt.M(t, ticket.FieldValues).HasKey("priority")
	gt.V(t, ticket.FieldValues["priority"].Value).Equal(any("high"))
}

func TestTicketUseCase_Create_UnknownWorkspace(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	_, err := uc.Create(ctx, "nonexistent", "Title", "", "", "", nil)
	gt.Error(t, err)
}

func TestTicketUseCase_GetAndList(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created := gt.R1(uc.Create(ctx, "ws-test", "T1", "", "", "", nil)).NoError(t)

	got := gt.R1(uc.Get(ctx, "ws-test", created.ID)).NoError(t)
	gt.S(t, got.Title).Equal("T1")

	tickets := gt.R1(uc.List(ctx, "ws-test", nil, nil)).NoError(t)
	gt.A(t, tickets).Length(1)
}

func TestTicketUseCase_List_FilterByClosed(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	gt.R1(uc.Create(ctx, "ws-test", "Open Ticket", "", "open", "", nil)).NoError(t)
	gt.R1(uc.Create(ctx, "ws-test", "Resolved Ticket", "", "resolved", "", nil)).NoError(t)
	gt.R1(uc.Create(ctx, "ws-test", "Closed Ticket", "", "closed", "", nil)).NoError(t)

	isClosed := true
	closedTickets := gt.R1(uc.List(ctx, "ws-test", &isClosed, nil)).NoError(t)
	gt.A(t, closedTickets).Length(2)

	isOpen := false
	openTickets := gt.R1(uc.List(ctx, "ws-test", &isOpen, nil)).NoError(t)
	gt.A(t, openTickets).Length(1)
}

func TestTicketUseCase_Update(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created := gt.R1(uc.Create(ctx, "ws-test", "Original", "desc", "", "", nil)).NoError(t)

	newTitle := "Updated"
	newStatus := types.StatusID("in-progress")
	updated := gt.R1(uc.Update(ctx, "ws-test", created.ID, &newTitle, nil, &newStatus, nil, nil)).NoError(t)
	gt.S(t, updated.Title).Equal("Updated")
	gt.S(t, string(updated.StatusID)).Equal("in-progress")
	gt.S(t, updated.Description).Equal("desc")
}

func TestTicketUseCase_Update_MergeFields(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	initial := map[string]model.FieldValue{
		"priority": {FieldID: "priority", Value: "high"},
	}
	created := gt.R1(uc.Create(ctx, "ws-test", "T", "", "", "", initial)).NoError(t)

	newFields := map[string]model.FieldValue{
		"category": {FieldID: "category", Value: "bug"},
	}
	updated := gt.R1(uc.Update(ctx, "ws-test", created.ID, nil, nil, nil, nil, newFields)).NoError(t)
	gt.M(t, updated.FieldValues).HasKey("priority")
	gt.M(t, updated.FieldValues).HasKey("category")
}

func TestTicketUseCase_Delete(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created := gt.R1(uc.Create(ctx, "ws-test", "To Delete", "", "", "", nil)).NoError(t)
	gt.NoError(t, uc.Delete(ctx, "ws-test", created.ID))

	_, err := uc.Get(ctx, "ws-test", created.ID)
	gt.Error(t, err)
}

func TestTicketUseCase_Create_RecordsHistory(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "History Test", "", "", "", nil)).NoError(t)

	histories := gt.R1(uc.ListHistory(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.A(t, histories).Length(1)
	gt.S(t, histories[0].Action).Equal("created")
	gt.S(t, string(histories[0].NewStatusID)).Equal("open")
	gt.S(t, string(histories[0].OldStatusID)).Equal("")
	gt.S(t, string(histories[0].ChangedBy)).Equal("system")
}

func TestTicketUseCase_Update_StatusChange_RecordsHistory(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "Status Change", "", "", "", nil)).NoError(t)

	newStatus := types.StatusID("in-progress")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &newStatus, nil, nil)).NoError(t)

	histories := gt.R1(uc.ListHistory(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.A(t, histories).Length(2)
	gt.S(t, histories[0].Action).Equal("created")
	gt.S(t, histories[1].Action).Equal("changed")
	gt.S(t, string(histories[1].OldStatusID)).Equal("open")
	gt.S(t, string(histories[1].NewStatusID)).Equal("in-progress")
}

func TestTicketUseCase_Update_NoStatusChange_NoHistory(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "No Status Change", "", "", "", nil)).NoError(t)

	newTitle := "Updated Title"
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, &newTitle, nil, nil, nil, nil)).NoError(t)

	histories := gt.R1(uc.ListHistory(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.A(t, histories).Length(1) // only the "created" entry
}
