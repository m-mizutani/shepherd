package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func newTriageTicket(t *testing.T, repo interfaces.Repository, ws types.WorkspaceID) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)
	ticket := &model.Ticket{
		ID:          types.TicketID(uuid.Must(uuid.NewV7()).String()),
		WorkspaceID: ws,
		Title:       "triage test",
		StatusID:    "open",
		FieldValues: make(map[string]model.FieldValue),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return gt.R1(repo.Ticket().Create(ctx, ws, ticket)).NoError(t)
}

func TestFinalizeTriage_Assigned(t *testing.T) {
	runTest(t, "Assigned", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		ws := types.WorkspaceID("ws-finalize-assigned")
		tk := newTriageTicket(t, repo, ws)
		assignees := []types.SlackUserID{"U123", "U456"}

		err := repo.Ticket().FinalizeTriage(ctx, ws, tk.ID, &assignees, &model.TicketHistory{
			Action:    "triage_completed",
			ChangedBy: assignees[0],
		})
		gt.NoError(t, err)

		got := gt.R1(repo.Ticket().Get(ctx, ws, tk.ID)).NoError(t)
		gt.True(t, got.Triaged)
		gt.A(t, got.AssigneeIDs).Length(2)
		gt.S(t, string(got.AssigneeIDs[0])).Equal("U123")
		gt.S(t, string(got.AssigneeIDs[1])).Equal("U456")

		histories := gt.R1(repo.TicketHistory().List(ctx, ws, tk.ID)).NoError(t)
		gt.N(t, len(histories)).Equal(1)
		gt.S(t, histories[0].Action).Equal("triage_completed")
		gt.S(t, histories[0].ID).NotEqual("")
	})
}

func TestFinalizeTriage_Unassigned(t *testing.T) {
	runTest(t, "Unassigned", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		ws := types.WorkspaceID("ws-finalize-unassigned")
		tk := newTriageTicket(t, repo, ws)

		err := repo.Ticket().FinalizeTriage(ctx, ws, tk.ID, nil, &model.TicketHistory{
			Action: "triage_completed",
		})
		gt.NoError(t, err)

		got := gt.R1(repo.Ticket().Get(ctx, ws, tk.ID)).NoError(t)
		gt.True(t, got.Triaged)
		gt.A(t, got.AssigneeIDs).Length(0) // assignees untouched (nil pointer means leave alone)

		histories := gt.R1(repo.TicketHistory().List(ctx, ws, tk.ID)).NoError(t)
		gt.N(t, len(histories)).Equal(1)
	})
}

func TestFinalizeTriage_Idempotent(t *testing.T) {
	runTest(t, "Idempotent", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		ws := types.WorkspaceID("ws-finalize-idempotent")
		tk := newTriageTicket(t, repo, ws)
		assignees := []types.SlackUserID{"U1"}

		// First call: should record one history.
		gt.NoError(t, repo.Ticket().FinalizeTriage(ctx, ws, tk.ID, &assignees, &model.TicketHistory{
			Action: "triage_completed",
		}))

		// Second call on already-triaged ticket: must be a no-op (no extra history).
		gt.NoError(t, repo.Ticket().FinalizeTriage(ctx, ws, tk.ID, &assignees, &model.TicketHistory{
			Action: "triage_completed_again",
		}))

		histories := gt.R1(repo.TicketHistory().List(ctx, ws, tk.ID)).NoError(t)
		gt.N(t, len(histories)).Equal(1)
		gt.S(t, histories[0].Action).Equal("triage_completed")
	})
}

func TestFinalizeTriage_NotFound(t *testing.T) {
	runTest(t, "NotFound", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		ws := types.WorkspaceID("ws-finalize-notfound")

		err := repo.Ticket().FinalizeTriage(ctx, ws, types.TicketID("does-not-exist"), nil, &model.TicketHistory{
			Action: "triage_completed",
		})
		gt.Error(t, err)
	})
}

func TestFinalizeTriage_NilHistory(t *testing.T) {
	runTest(t, "NilHistory", func(t *testing.T, repo interfaces.Repository) {
		ctx := context.Background()
		ws := types.WorkspaceID("ws-finalize-nil")
		tk := newTriageTicket(t, repo, ws)

		err := repo.Ticket().FinalizeTriage(ctx, ws, tk.ID, nil, nil)
		gt.Error(t, err)

		// ticket should remain untriaged
		got := gt.R1(repo.Ticket().Get(ctx, ws, tk.ID)).NoError(t)
		gt.False(t, got.Triaged)
	})
}
