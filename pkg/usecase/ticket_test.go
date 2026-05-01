package usecase_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
)

type fakeTicketChangeNotifier struct {
	calls            []fakeTicketChangeCall
	conclusionCalls  []fakeConclusionCall
	conclusionErr    error
}

type fakeTicketChangeCall struct {
	channelID string
	threadTS  string
	change    slackService.TicketChange
}

type fakeConclusionCall struct {
	channelID  string
	threadTS   string
	conclusion string
}

func (f *fakeTicketChangeNotifier) NotifyTicketChange(_ context.Context, channelID, threadTS string, change slackService.TicketChange) error {
	f.calls = append(f.calls, fakeTicketChangeCall{
		channelID: channelID,
		threadTS:  threadTS,
		change:    change,
	})
	return nil
}

func (f *fakeTicketChangeNotifier) PostConclusion(_ context.Context, channelID, threadTS, conclusion string) error {
	f.conclusionCalls = append(f.conclusionCalls, fakeConclusionCall{
		channelID:  channelID,
		threadTS:   threadTS,
		conclusion: conclusion,
	})
	return f.conclusionErr
}

func setupTicketUseCase(t *testing.T) (*usecase.TicketUseCase, *model.WorkspaceRegistry) {
	uc, _, _, registry := setupTicketUseCaseFull(t, nil)
	return uc, registry
}

func setupTicketUseCaseFull(t *testing.T, notifier usecase.TicketChangeNotifier) (*usecase.TicketUseCase, interfaces.Repository, *fakeTicketChangeNotifier, *model.WorkspaceRegistry) {
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

	var fake *fakeTicketChangeNotifier
	if notifier == nil {
		fake = &fakeTicketChangeNotifier{}
		notifier = fake
	}

	uc := usecase.NewTicketUseCase(repo, registry, notifier, nil)
	return uc, repo, fake, registry
}

func TestTicketUseCase_Create(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "My Ticket", "desc", "", nil, nil)).NoError(t)
	gt.S(t, ticket.Title).Equal("My Ticket")
	gt.S(t, string(ticket.StatusID)).Equal("open")
	gt.S(t, string(ticket.ID)).NotEqual("")
}

func TestTicketUseCase_Create_WithStatus(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "Custom Status", "", "in-progress", nil, nil)).NoError(t)
	gt.S(t, string(ticket.StatusID)).Equal("in-progress")
}

func TestTicketUseCase_Create_WithFields(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	fields := map[string]model.FieldValue{
		"priority": {FieldID: "priority", Type: types.FieldTypeSelect, Value: "high"},
	}
	ticket := gt.R1(uc.Create(ctx, "ws-test", "With Fields", "", "", nil, fields)).NoError(t)
	gt.M(t, ticket.FieldValues).HasKey("priority")
	gt.V(t, ticket.FieldValues["priority"].Value).Equal(any("high"))
}

func TestTicketUseCase_Create_UnknownWorkspace(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	_, err := uc.Create(ctx, "nonexistent", "Title", "", "", nil, nil)
	gt.Error(t, err)
}

func TestTicketUseCase_GetAndList(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created := gt.R1(uc.Create(ctx, "ws-test", "T1", "", "", nil, nil)).NoError(t)

	got := gt.R1(uc.Get(ctx, "ws-test", created.ID)).NoError(t)
	gt.S(t, got.Title).Equal("T1")

	tickets := gt.R1(uc.List(ctx, "ws-test", nil, nil)).NoError(t)
	gt.A(t, tickets).Length(1)
}

func TestTicketUseCase_List_FilterByClosed(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	gt.R1(uc.Create(ctx, "ws-test", "Open Ticket", "", "open", nil, nil)).NoError(t)
	gt.R1(uc.Create(ctx, "ws-test", "Resolved Ticket", "", "resolved", nil, nil)).NoError(t)
	gt.R1(uc.Create(ctx, "ws-test", "Closed Ticket", "", "closed", nil, nil)).NoError(t)

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

	created := gt.R1(uc.Create(ctx, "ws-test", "Original", "desc", "", nil, nil)).NoError(t)

	newTitle := "Updated"
	newStatus := types.StatusID("in-progress")
	updated := gt.R1(uc.Update(ctx, "ws-test", created.ID, &newTitle, nil, &newStatus, nil, nil, nil)).NoError(t)
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
	created := gt.R1(uc.Create(ctx, "ws-test", "T", "", "", nil, initial)).NoError(t)

	newFields := map[string]model.FieldValue{
		"category": {FieldID: "category", Value: "bug"},
	}
	updated := gt.R1(uc.Update(ctx, "ws-test", created.ID, nil, nil, nil, nil, newFields, nil)).NoError(t)
	gt.M(t, updated.FieldValues).HasKey("priority")
	gt.M(t, updated.FieldValues).HasKey("category")
}

func TestTicketUseCase_Delete(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	created := gt.R1(uc.Create(ctx, "ws-test", "To Delete", "", "", nil, nil)).NoError(t)
	gt.NoError(t, uc.Delete(ctx, "ws-test", created.ID))

	_, err := uc.Get(ctx, "ws-test", created.ID)
	gt.Error(t, err)
}

func TestTicketUseCase_Create_RecordsHistory(t *testing.T) {
	uc, _ := setupTicketUseCase(t)
	ctx := context.Background()

	ticket := gt.R1(uc.Create(ctx, "ws-test", "History Test", "", "", nil, nil)).NoError(t)

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

	ticket := gt.R1(uc.Create(ctx, "ws-test", "Status Change", "", "", nil, nil)).NoError(t)

	newStatus := types.StatusID("in-progress")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &newStatus, nil, nil, nil)).NoError(t)

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

	ticket := gt.R1(uc.Create(ctx, "ws-test", "No Status Change", "", "", nil, nil)).NoError(t)

	newTitle := "Updated Title"
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, &newTitle, nil, nil, nil, nil, nil)).NoError(t)

	histories := gt.R1(uc.ListHistory(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.A(t, histories).Length(1) // only the "created" entry
}

// seedSlackTicket creates a ticket and immediately attaches Slack
// metadata to it via the repository so subsequent Update calls go
// through the Slack notifier path. The metadata seed itself bypasses
// the usecase to avoid a spurious notification on a freshly created
// ticket that has no observable status / assignee transition yet.
func seedSlackTicket(t *testing.T, uc *usecase.TicketUseCase, repo interfaces.Repository, assignees []types.SlackUserID) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	created := gt.R1(uc.Create(ctx, "ws-test", "Seeded", "desc", "", assignees, nil)).NoError(t)
	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", created.ID)).NoError(t)
	got.SlackChannelID = "C111"
	got.SlackThreadTS = "1700000000.000100"
	updated := gt.R1(repo.Ticket().Update(ctx, "ws-test", got)).NoError(t)
	return updated
}

func TestTicketUseCase_Update_StatusChange_NotifiesOnce(t *testing.T) {
	uc, repo, notifier, _ := setupTicketUseCaseFull(t, nil)
	ctx := context.Background()

	ticket := seedSlackTicket(t, uc, repo, nil)

	newStatus := types.StatusID("in-progress")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &newStatus, nil, nil, nil)).NoError(t)

	gt.A(t, notifier.calls).Length(1)
	call := notifier.calls[0]
	gt.S(t, call.channelID).Equal("C111")
	gt.S(t, call.threadTS).Equal("1700000000.000100")
	gt.V(t, call.change.StatusChanged).Equal(true)
	gt.V(t, call.change.AssigneeChanged).Equal(false)
	gt.S(t, call.change.OldStatusName).Equal("Open")
	gt.S(t, call.change.NewStatusName).Equal("In Progress")
}

func TestTicketUseCase_Update_AssigneeChange_NotifiesOnce(t *testing.T) {
	uc, repo, notifier, _ := setupTicketUseCaseFull(t, nil)
	ctx := context.Background()

	ticket := seedSlackTicket(t, uc, repo, nil)

	newAssignees := []types.SlackUserID{"U111", "U222"}
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, nil, &newAssignees, nil, nil)).NoError(t)

	gt.A(t, notifier.calls).Length(1)
	call := notifier.calls[0]
	gt.V(t, call.change.StatusChanged).Equal(false)
	gt.V(t, call.change.AssigneeChanged).Equal(true)
	gt.A(t, call.change.OldAssigneeIDs).Length(0)
	gt.A(t, call.change.NewAssigneeIDs).Length(2)
	gt.S(t, call.change.NewAssigneeIDs[0]).Equal("U111")
}

func TestTicketUseCase_Update_StatusAndAssignee_NotifiesOnce(t *testing.T) {
	uc, repo, notifier, _ := setupTicketUseCaseFull(t, nil)
	ctx := context.Background()

	ticket := seedSlackTicket(t, uc, repo, []types.SlackUserID{"U000"})

	newStatus := types.StatusID("resolved")
	newAssignees := []types.SlackUserID{"U999"}
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &newStatus, &newAssignees, nil, nil)).NoError(t)

	gt.A(t, notifier.calls).Length(1)
	call := notifier.calls[0]
	gt.V(t, call.change.StatusChanged).Equal(true)
	gt.V(t, call.change.AssigneeChanged).Equal(true)
	gt.S(t, call.change.NewStatusName).Equal("Resolved")
	gt.A(t, call.change.OldAssigneeIDs).Length(1)
	gt.S(t, call.change.OldAssigneeIDs[0]).Equal("U000")
	gt.S(t, call.change.NewAssigneeIDs[0]).Equal("U999")
}

func TestTicketUseCase_Update_NoChange_NoNotify(t *testing.T) {
	uc, repo, notifier, _ := setupTicketUseCaseFull(t, nil)
	ctx := context.Background()

	ticket := seedSlackTicket(t, uc, repo, []types.SlackUserID{"U000"})

	// Title-only change must not fire the notifier.
	newTitle := "Renamed"
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, &newTitle, nil, nil, nil, nil, nil)).NoError(t)

	gt.A(t, notifier.calls).Length(0)
}

func TestTicketUseCase_Update_AssigneeReorder_NoNotify(t *testing.T) {
	uc, repo, notifier, _ := setupTicketUseCaseFull(t, nil)
	ctx := context.Background()

	ticket := seedSlackTicket(t, uc, repo, []types.SlackUserID{"U001", "U002"})

	// Same set, different order — assignee membership is unchanged.
	reordered := []types.SlackUserID{"U002", "U001"}
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, nil, &reordered, nil, nil)).NoError(t)

	gt.A(t, notifier.calls).Length(0)
}

// fixedConclusionLLM returns an LLMClientMock whose session emits the given
// conclusion JSON exactly once. errOnGenerate, when non-nil, causes the
// session's Generate to fail with that error so callers can exercise the
// "LLM failed" branch.
func fixedConclusionLLM(conclusion string, errOnGenerate error) *mock.LLMClientMock {
	session := &mock.SessionMock{
		GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			if errOnGenerate != nil {
				return nil, errOnGenerate
			}
			return &gollem.Response{Texts: []string{`{"conclusion":"` + conclusion + `"}`}}, nil
		},
		HistoryFunc:       func() (*gollem.History, error) { return &gollem.History{LLType: gollem.LLMTypeOpenAI, Version: gollem.HistoryVersion}, nil },
		AppendHistoryFunc: func(_ *gollem.History) error { return nil },
	}
	return &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}
}

func setupTicketUseCaseWithLLM(t *testing.T, llm gollem.LLMClient) (*usecase.TicketUseCase, interfaces.Repository, *fakeTicketChangeNotifier) {
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
	notifier := &fakeTicketChangeNotifier{}
	uc := usecase.NewTicketUseCase(repo, registry, notifier, llm)
	return uc, repo, notifier
}

func seedClosableTicket(t *testing.T, uc *usecase.TicketUseCase, repo interfaces.Repository) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	created := gt.R1(uc.Create(ctx, "ws-test", "Login broken", "Safari users see blank page", "open", nil, nil)).NoError(t)

	stored := gt.R1(repo.Ticket().Get(ctx, "ws-test", created.ID)).NoError(t)
	stored.SlackChannelID = "C-thread"
	stored.SlackThreadTS = "1700000000.000100"
	stored.InitialMessage = "Hi, login is broken on Safari."
	stored = gt.R1(repo.Ticket().Update(ctx, "ws-test", stored)).NoError(t)

	gt.R1(repo.Comment().Create(ctx, "ws-test", stored.ID, &model.Comment{
		ID: "cmt-1", TicketID: stored.ID, SlackUserID: "U_REPORTER", Body: "Repro on Safari 17", SlackTS: "1700000001.000000",
	})).NoError(t)
	gt.R1(repo.Comment().Create(ctx, "ws-test", stored.ID, &model.Comment{
		ID: "cmt-2", TicketID: stored.ID, IsBot: true, Body: "Investigating CSP violations", SlackTS: "1700000002.000000",
	})).NoError(t)
	gt.R1(repo.Comment().Create(ctx, "ws-test", stored.ID, &model.Comment{
		ID: "cmt-3", TicketID: stored.ID, SlackUserID: "U_OWNER", Body: "Patch deployed", SlackTS: "1700000003.000000",
	})).NoError(t)
	return stored
}

func TestTicketUseCase_Update_GeneratesConclusionOnClose(t *testing.T) {
	llm := fixedConclusionLLM("Login was broken on Safari due to a CSP violation; resolved by U_OWNER's patch.", nil)
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()

	ticket := seedClosableTicket(t, uc, repo)

	closed := types.StatusID("closed")
	updated := gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	gt.S(t, string(updated.StatusID)).Equal("closed")

	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("Login was broken on Safari due to a CSP violation; resolved by U_OWNER's patch.")

	gt.A(t, notifier.conclusionCalls).Length(1)
	call := notifier.conclusionCalls[0]
	gt.S(t, call.channelID).Equal("C-thread")
	gt.S(t, call.threadTS).Equal("1700000000.000100")
	gt.S(t, call.conclusion).Equal("Login was broken on Safari due to a CSP violation; resolved by U_OWNER's patch.")
}

func TestTicketUseCase_Update_NoConclusionForNonClosingTransition(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM must not be invoked for non-closing transitions")
			return nil, nil
		},
	}
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()

	ticket := seedClosableTicket(t, uc, repo)

	inProgress := types.StatusID("in-progress")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &inProgress, nil, nil, nil)).NoError(t)
	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("")
	gt.A(t, notifier.conclusionCalls).Length(0)
}

func TestTicketUseCase_Update_RegeneratesConclusionOnReopenThenClose(t *testing.T) {
	first := fixedConclusionLLM("First close summary", nil)
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, first)
	ctx := context.Background()

	ticket := seedClosableTicket(t, uc, repo)
	closed := types.StatusID("closed")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	async.Wait()

	open := types.StatusID("open")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &open, nil, nil, nil)).NoError(t)
	async.Wait()

	mid := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, mid.Conclusion).Equal("First close summary")

	usecase.SetTicketUseCaseLLMForTest(uc, fixedConclusionLLM("Second close summary", nil))
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	async.Wait()

	final := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, final.Conclusion).Equal("Second close summary")
	gt.A(t, notifier.conclusionCalls).Length(2)
}

func TestTicketUseCase_Update_ConclusionEditOnNonClosedRejected(t *testing.T) {
	uc, repo, _ := setupTicketUseCaseWithLLM(t, nil)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	body := "Manual conclusion attempt"
	_, err := uc.Update(ctx, "ws-test", ticket.ID, nil, nil, nil, nil, nil, &body)
	gt.True(t, errors.Is(err, usecase.ErrConclusionEditNotAllowed))

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("")
}

func TestTicketUseCase_Update_ConclusionEditOnClosedSucceeds(t *testing.T) {
	uc, repo, _ := setupTicketUseCaseWithLLM(t, nil)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	stored := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	stored.StatusID = "closed"
	gt.R1(repo.Ticket().Update(ctx, "ws-test", stored)).NoError(t)

	body := "Manually authored conclusion"
	updated := gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, nil, nil, nil, &body)).NoError(t)
	gt.S(t, updated.Conclusion).Equal("Manually authored conclusion")

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("Manually authored conclusion")
}

func TestTicketUseCase_Update_StatusAndConclusionTogetherRejected(t *testing.T) {
	uc, repo, _ := setupTicketUseCaseWithLLM(t, nil)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	stored := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	stored.StatusID = "closed"
	gt.R1(repo.Ticket().Update(ctx, "ws-test", stored)).NoError(t)

	body := "Manual"
	closed := types.StatusID("closed")
	_, err := uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, &body)
	gt.True(t, errors.Is(err, usecase.ErrConclusionEditNotAllowed))
}

func TestTicketUseCase_Update_LLMFailureKeepsClose(t *testing.T) {
	llm := fixedConclusionLLM("", goerr.New("simulated LLM outage"))
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	closed := types.StatusID("closed")
	updated := gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	gt.S(t, string(updated.StatusID)).Equal("closed")

	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("")

	if len(notifier.calls) == 0 {
		t.Errorf("notifyTicketChange must still fire even when conclusion generation fails")
	}
	gt.A(t, notifier.conclusionCalls).Length(0)
}

// scriptedConclusionLLM returns an LLMClientMock whose session emits a
// pre-recorded sequence of responses (one per Generate call). Each entry
// is the raw text the model returns; tests use this to script "first
// attempt malformed → retry succeeds" lifecycles. After the scripted
// turns are exhausted, further calls fail the test.
func scriptedConclusionLLM(t *testing.T, responses []string) (*mock.LLMClientMock, func() int) {
	t.Helper()
	var calls int32
	session := &mock.SessionMock{
		GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			n := int(atomic.AddInt32(&calls, 1))
			if n > len(responses) {
				t.Fatalf("LLM was invoked more than the scripted %d times (call %d)", len(responses), n)
				return nil, nil
			}
			return &gollem.Response{Texts: []string{responses[n-1]}}, nil
		},
		HistoryFunc:       func() (*gollem.History, error) { return &gollem.History{LLType: gollem.LLMTypeOpenAI, Version: gollem.HistoryVersion}, nil },
		AppendHistoryFunc: func(_ *gollem.History) error { return nil },
	}
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}
	return llm, func() int { return int(atomic.LoadInt32(&calls)) }
}

func TestTicketUseCase_Update_RetriesOnMalformedJSONThenSucceeds(t *testing.T) {
	llm, callCount := scriptedConclusionLLM(t, []string{
		`not valid json at all`,
		`{"conclusion":"Recovered after one retry."}`,
	})
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	closed := types.StatusID("closed")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("Recovered after one retry.")
	gt.A(t, notifier.conclusionCalls).Length(1)
	gt.Equal(t, callCount(), 2)
}

func TestTicketUseCase_Update_RetriesOnEmptyConclusionThenSucceeds(t *testing.T) {
	llm, callCount := scriptedConclusionLLM(t, []string{
		`{"conclusion":""}`,
		`{"conclusion":"   "}`,
		`{"conclusion":"Third time is the charm."}`,
	})
	uc, repo, _ := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	closed := types.StatusID("closed")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("Third time is the charm.")
	gt.Equal(t, callCount(), 3)
}

func TestTicketUseCase_Update_RetriesExhaustedKeepsConclusionEmpty(t *testing.T) {
	// 4 malformed responses = initial attempt + 3 retries (the cap).
	llm, callCount := scriptedConclusionLLM(t, []string{
		`garbage 1`,
		`garbage 2`,
		`garbage 3`,
		`garbage 4`,
	})
	uc, repo, notifier := setupTicketUseCaseWithLLM(t, llm)
	ctx := context.Background()
	ticket := seedClosableTicket(t, uc, repo)

	closed := types.StatusID("closed")
	gt.R1(uc.Update(ctx, "ws-test", ticket.ID, nil, nil, &closed, nil, nil, nil)).NoError(t)
	async.Wait()

	got := gt.R1(repo.Ticket().Get(ctx, "ws-test", ticket.ID)).NoError(t)
	gt.S(t, got.Conclusion).Equal("")
	gt.A(t, notifier.conclusionCalls).Length(0)
	gt.Equal(t, callCount(), 4)
}
