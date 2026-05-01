package usecase_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
)

func setupQuickActionsRig(t *testing.T) (*usecase.QuickActionsUseCase, *memory.Repository, *fakeTicketChangeNotifier) {
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
			},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
			},
		},
		SlackChannelID: "C-quick",
	})

	notifier := &fakeTicketChangeNotifier{}
	ticketUC := usecase.NewTicketUseCase(repo, registry, notifier, nil)
	uc := usecase.NewQuickActionsUseCase(repo, registry, ticketUC)
	return uc, repo, notifier
}

func plantQuickTicket(t *testing.T, repo *memory.Repository, threadTS string, assignees []types.SlackUserID, statusID types.StatusID) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	ticket := &model.Ticket{
		ID:             "tkt-q",
		WorkspaceID:    "ws-test",
		Title:          "T",
		StatusID:       statusID,
		AssigneeIDs:    assignees,
		SlackChannelID: "C-quick",
		SlackThreadTS:  types.SlackThreadTS(threadTS),
	}
	created := gt.R1(repo.Ticket().Create(ctx, "ws-test", ticket)).NoError(t)
	return created
}

func TestQuickActionsUseCase_HandleStatusChange_UpdatesAndNotifies(t *testing.T) {
	uc, repo, notifier := setupQuickActionsRig(t)
	plantQuickTicket(t, repo, "100.000", nil, "open")

	ctx := context.Background()
	gt.NoError(t, uc.HandleStatusChange(ctx, "C-quick", "100.000", "in-progress"))

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", "tkt-q")).NoError(t)
	gt.S(t, string(got.StatusID)).Equal("in-progress")

	gt.A(t, notifier.calls).Length(1)
	gt.V(t, notifier.calls[0].change.StatusChanged).Equal(true)
	gt.V(t, notifier.calls[0].change.AssigneeChanged).Equal(false)
}

func TestQuickActionsUseCase_HandleAssigneeChange_UpdatesAndNotifies(t *testing.T) {
	uc, repo, notifier := setupQuickActionsRig(t)
	plantQuickTicket(t, repo, "200.000", nil, "open")

	ctx := context.Background()
	gt.NoError(t, uc.HandleAssigneeChange(ctx, "C-quick", "200.000", []string{"U-alice", "U-bob"}))

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", "tkt-q")).NoError(t)
	gt.A(t, got.AssigneeIDs).Length(2)

	gt.A(t, notifier.calls).Length(1)
	gt.V(t, notifier.calls[0].change.AssigneeChanged).Equal(true)
	gt.A(t, notifier.calls[0].change.NewAssigneeIDs).Length(2)
}

func TestQuickActionsUseCase_HandleAssigneeChange_ClearAssignees(t *testing.T) {
	uc, repo, notifier := setupQuickActionsRig(t)
	plantQuickTicket(t, repo, "300.000", []types.SlackUserID{"U-prev"}, "open")

	ctx := context.Background()
	gt.NoError(t, uc.HandleAssigneeChange(ctx, "C-quick", "300.000", []string{}))

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", "tkt-q")).NoError(t)
	gt.A(t, got.AssigneeIDs).Length(0)

	gt.A(t, notifier.calls).Length(1)
	gt.A(t, notifier.calls[0].change.OldAssigneeIDs).Length(1)
	gt.A(t, notifier.calls[0].change.NewAssigneeIDs).Length(0)
}

func TestQuickActionsUseCase_HandleStatusChange_NoChange_NoNotify(t *testing.T) {
	uc, repo, notifier := setupQuickActionsRig(t)
	plantQuickTicket(t, repo, "400.000", nil, "open")

	ctx := context.Background()
	gt.NoError(t, uc.HandleStatusChange(ctx, "C-quick", "400.000", "open"))

	gt.A(t, notifier.calls).Length(0)
}

func TestQuickActionsUseCase_UnknownChannel_Noop(t *testing.T) {
	uc, _, notifier := setupQuickActionsRig(t)

	ctx := context.Background()
	gt.NoError(t, uc.HandleStatusChange(ctx, "C-not-mapped", "100.000", "open"))
	gt.NoError(t, uc.HandleAssigneeChange(ctx, "C-not-mapped", "100.000", []string{"U-x"}))

	gt.A(t, notifier.calls).Length(0)
}

func TestQuickActionsUseCase_UnknownThread_Noop(t *testing.T) {
	uc, _, notifier := setupQuickActionsRig(t)

	ctx := context.Background()
	gt.NoError(t, uc.HandleStatusChange(ctx, "C-quick", "999.999", "open"))
	gt.NoError(t, uc.HandleAssigneeChange(ctx, "C-quick", "999.999", []string{"U-x"}))

	gt.A(t, notifier.calls).Length(0)
}

func setupQuickActionsRigWithLLM(t *testing.T, llm gollem.LLMClient) (*usecase.QuickActionsUseCase, *memory.Repository, *fakeTicketChangeNotifier) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: "ws-test", Name: "Test"},
		FieldSchema: &config.FieldSchema{
			Statuses: []config.StatusDef{
				{ID: "open", Name: "Open"},
				{ID: "closed", Name: "Closed"},
			},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
				ClosedStatusIDs: []types.StatusID{"closed"},
			},
		},
		SlackChannelID: "C-quick",
	})

	notifier := &fakeTicketChangeNotifier{}
	ticketUC := usecase.NewTicketUseCase(repo, registry, notifier, llm)
	uc := usecase.NewQuickActionsUseCase(repo, registry, ticketUC)
	return uc, repo, notifier
}

// TestLifecycle_QuickActions_StatusChangeToClose_GeneratesConclusion proves
// that the entry-point unification rule holds for the Slack QuickActions
// path: a status flip via the Slack-side handler triggers the same
// conclusion-generation tail as a Web PATCH would, with no per-handler
// duplication of the close-detection logic.
func TestLifecycle_QuickActions_StatusChangeToClose_GeneratesConclusion(t *testing.T) {
	uc, repo, notifier := setupQuickActionsRigWithLLM(t, fixedConclusionLLM("Closed via Slack quick actions.", nil))
	plantQuickTicket(t, repo, "500.000", nil, "open")

	ctx := context.Background()
	gt.NoError(t, uc.HandleStatusChange(ctx, "C-quick", "500.000", "closed"))
	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", "tkt-q")).NoError(t)
	gt.S(t, string(got.StatusID)).Equal("closed")
	gt.S(t, got.Conclusion).Equal("Closed via Slack quick actions.")

	gt.A(t, notifier.conclusionCalls).Length(1)
	gt.S(t, notifier.conclusionCalls[0].channelID).Equal("C-quick")
	gt.S(t, notifier.conclusionCalls[0].threadTS).Equal("500.000")
	gt.S(t, notifier.conclusionCalls[0].conclusion).Equal("Closed via Slack quick actions.")
}
