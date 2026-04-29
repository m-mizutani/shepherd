package triage_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	slackgo "github.com/slack-go/slack"
)

// fakeTriageSlack records every Slack call the triage executor makes through
// SlackTriageClient so tests can assert exact ordering / payloads.
type fakeTriageSlack struct {
	mu        sync.Mutex
	posts     []postCall
	updates   []updateCall
	replies   []replyCall
	views     []viewCall
	postErr   error
	updateErr error
	viewErr   error
}

type viewCall struct {
	triggerID  string
	callbackID string
	view       slackgo.ModalViewRequest
}

type postCall struct {
	channel  string
	threadTS string
	blocks   []slackgo.Block
}
type updateCall struct {
	channel   string
	messageTS string
	blocks    []slackgo.Block
}
type replyCall struct {
	channel  string
	threadTS string
	text     string
}

func (f *fakeTriageSlack) PostThreadBlocks(_ context.Context, channel, threadTS string, blocks []slackgo.Block) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.postErr != nil {
		return "", f.postErr
	}
	f.posts = append(f.posts, postCall{channel, threadTS, blocks})
	return "ts-" + threadTS, nil
}

func (f *fakeTriageSlack) UpdateMessage(_ context.Context, channel, messageTS string, blocks []slackgo.Block) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updates = append(f.updates, updateCall{channel, messageTS, blocks})
	return nil
}

func (f *fakeTriageSlack) ReplyThread(_ context.Context, channel, threadTS, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, replyCall{channel, threadTS, text})
	return nil
}

func (f *fakeTriageSlack) PostEphemeral(_ context.Context, _, _, _ string) error { return nil }

func (f *fakeTriageSlack) OpenView(_ context.Context, triggerID string, view slackgo.ModalViewRequest) (*slackgo.ViewResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.viewErr != nil {
		return nil, f.viewErr
	}
	f.views = append(f.views, viewCall{triggerID: triggerID, callbackID: view.CallbackID, view: view})
	return &slackgo.ViewResponse{}, nil
}

const (
	tWS      = types.WorkspaceID("ws-tri")
	tChannel = "C-triage"
	tThread  = "1000.000"
)

// rig wires a usecase + executor against in-memory + fakes. The supplied LLM
// (or nil) is used for the dispatched Run; tests that should not invoke the
// LLM pass nil and assert no Slack post/update happens that would only occur
// after a successful llmPlan.
func newRig(t *testing.T, llm gollem.LLMClient) (*triage.UseCase, *triage.PlanExecutor, *memory.Repository, *fakeHistoryRepo, *fakeTriageSlack) {
	t.Helper()
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	hist := newFakeHistory()
	slack := &fakeTriageSlack{}
	catalog := tool.NewCatalog(nil, repo.ToolSettings())
	promptUC := prompt.New(repo.Prompt())
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, catalog, promptUC, nil, triage.Config{IterationCap: 5})
	uc := triage.NewUseCase(exec, &fakeResolver{ws: tWS, channel: tChannel})
	return uc, exec, repo, hist, slack
}

type fakeResolver struct {
	ws      types.WorkspaceID
	channel string
}

func (r *fakeResolver) ResolveWorkspace(channelID string) (types.WorkspaceID, bool) {
	if channelID == r.channel {
		return r.ws, true
	}
	return "", false
}

func mustCreateTicket(t *testing.T, repo *memory.Repository, triaged bool) *model.Ticket {
	t.Helper()
	ctx := context.Background()
	ticket, err := repo.Ticket().Create(ctx, tWS, &model.Ticket{
		WorkspaceID:         tWS,
		Title:               "Login broken",
		Description:         "Users report 500s on login",
		ReporterSlackUserID: "Ureporter",
		SlackChannelID:      types.SlackChannelID(tChannel),
		SlackThreadTS:       types.SlackThreadTS(tThread),
		Triaged:             triaged,
	})
	gt.NoError(t, err)
	return ticket
}

func seedAskHistory(t *testing.T, hist *fakeHistoryRepo, ticketID types.TicketID) {
	t.Helper()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, askPlanJSON))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticketID), h))
}

// stateFor builds a *slackgo.BlockActionStates that picks the given choice for
// "q1". When choiceID is empty, the state has the block_id but no selected
// option (used for validation failures).
func stateFor(choiceID string, otherText string) *slackgo.BlockActionStates {
	state := &slackgo.BlockActionStates{Values: map[string]map[string]slackgo.BlockAction{}}
	choice := slackgo.BlockAction{ActionID: slackService.TriageChoiceActionID}
	if choiceID != "" {
		choice.SelectedOption = slackgo.OptionBlockObject{Value: choiceID}
	}
	state.Values["q1"] = map[string]slackgo.BlockAction{slackService.TriageChoiceActionID: choice}
	if otherText != "" {
		state.Values["q1"+slackService.TriageOtherSuffix] = map[string]slackgo.BlockAction{
			slackService.TriageOtherTextActionID: {ActionID: slackService.TriageOtherTextActionID, Value: otherText},
		}
	}
	return state
}

func TestHandleSubmit_TicketAlreadyTriaged_Invalidates(t *testing.T) {
	uc, _, repo, _, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, true)

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-1",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	gt.S(t, slack.updates[0].messageTS).Equal("msg-1")
	gt.A(t, slack.posts).Length(0)
}

func TestHandleSubmit_NoPlanHistory_Invalidates(t *testing.T) {
	uc, _, repo, _, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-2",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	gt.A(t, slack.posts).Length(0)
}

func TestHandleSubmit_LatestPlanNotAsk_Invalidates(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)

	// Seed plan history with a propose_investigate (not an Ask).
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, investigatePlanJSON))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID), h))

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-3",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
}

func TestHandleSubmit_NotWaiting_Invalidates(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)

	// Seed: propose_ask followed by an already-recorded user response. That
	// is the "answers already submitted" condition — the form is stale.
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages,
		mustAssistantPlanMessage(t, askPlanJSON),
		mustUserTextMessage(t, "previous answer"),
	)
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID), h))

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-4",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
}

func TestHandleSubmit_ValidationError_RerendersForm(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	seedAskHistory(t, hist, ticket.ID)

	// Empty selection AND empty other text → invalid answer.
	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-5",
		State: stateFor("", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	// History must NOT have been appended with answers.
	hh, _ := hist.Load(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID))
	gt.NotNil(t, hh)
	for _, msg := range hh.Messages {
		if msg.Role == gollem.RoleUser {
			t.Fatalf("validation error path must not append a user-role message to history")
		}
	}
}

func TestHandleSubmit_HappyPath_AppendsAnswerAndAcksMessage(t *testing.T) {
	// Supply a stub LLM that errors out from NewSession; errutil.Handle in the
	// dispatched Run goroutine swallows the error. We only care about the
	// synchronous side effects of HandleSubmit here.
	stubLLM := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return nil, errors.New("stub: not configured")
		},
	}
	uc, _, repo, hist, slack := newRig(t, stubLLM)
	ticket := mustCreateTicket(t, repo, false)
	seedAskHistory(t, hist, ticket.ID)

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-6",
		State: stateFor("c1", ""),
	}))

	// Wait for the dispatched Run goroutine. With llm=nil the Run will fail
	// fast at llmPlan (logged via errutil.Handle) so this just drains the
	// goroutine without affecting the assertions on synchronous side
	// effects.
	async.Wait()

	// Slack: the original ask message was acked once with the "received"
	// notice. There may be additional updates from the dispatched Run; we
	// only assert the first one is on the right message.
	gt.True(t, len(slack.updates) >= 1)
	gt.S(t, slack.updates[0].messageTS).Equal("msg-6")

	// History: the formatted answers were appended as a user message.
	hh, _ := hist.Load(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID))
	gt.NotNil(t, hh)
	var userMessages []string
	for _, msg := range hh.Messages {
		if msg.Role != gollem.RoleUser {
			continue
		}
		for _, c := range msg.Contents {
			if c.Type == gollem.MessageContentTypeText {
				tc, err := c.GetTextContent()
				if err == nil {
					userMessages = append(userMessages, tc.Text)
				}
			}
		}
	}
	gt.A(t, userMessages).Length(1)
	gt.True(t, strings.Contains(userMessages[0], "What is the scope of impact?"))
	gt.True(t, strings.Contains(userMessages[0], "Production"))
}

func TestHandleSubmit_UnknownChannel_Invalidates(t *testing.T) {
	uc, _, repo, _, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)

	// ChannelID does not match the resolver's mapping → workspace lookup
	// fails → form invalidated, no DB or planner activity.
	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: "C-unknown", MessageTS: "msg-x",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	gt.S(t, slack.updates[0].messageTS).Equal("msg-x")
	gt.A(t, slack.posts).Length(0)
}

func TestHandleSubmit_UnknownTicket_Invalidates(t *testing.T) {
	uc, _, _, _, slack := newRig(t, nil)

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		TicketID:  types.TicketID("missing"),
		ChannelID: tChannel, MessageTS: "msg-y",
		State: stateFor("c1", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	gt.A(t, slack.posts).Length(0)
}
// fakeUserSlack records the calls SlackUseCase makes through the SlackClient
// surface. Distinct from fakeTriageSlack because the two interfaces share no
// methods and we want to assert on each side independently.
type fakeUserSlack struct {
	mu             sync.Mutex
	threadReplies  []replyCall
	ticketCreated  []ticketCreatedCall
}

type ticketCreatedCall struct {
	channelID string
	threadTS  string
	seqNum    int64
	ticketURL string
}

func (f *fakeUserSlack) ReplyThread(_ context.Context, ch, ts, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.threadReplies = append(f.threadReplies, replyCall{ch, ts, text})
	return nil
}

func (f *fakeUserSlack) ReplyTicketCreated(_ context.Context, ch, ts string, seq int64, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ticketCreated = append(f.ticketCreated, ticketCreatedCall{ch, ts, seq, url})
	return nil
}

func (f *fakeUserSlack) GetUserInfo(_ context.Context, id string) (*slackService.UserInfo, error) {
	return &slackService.UserInfo{ID: id, Name: id}, nil
}
func (f *fakeUserSlack) ListUsers(_ context.Context) ([]*slackService.UserInfo, error) {
	return nil, nil
}

// TestLifecycle_TicketCreate_Ask_Submit_Complete drives the full triage
// state machine end-to-end through the public entry points
// (HandleNewMessage and HandleSubmit), with no hand-rolled intermediate
// state. The LLM mock is sequenced so the first planner turn returns
// propose_ask and the second (resumed by Submit) returns propose_complete.
//
// Assertions are layered at every observable transition: agent history
// shape, Slack call ordering, and finally the persisted ticket fields.
func TestLifecycle_TicketCreate_Ask_Submit_Complete(t *testing.T) {
	const (
		channel  = "C-life"
		reporter = "Ureporter"
		threadTS = "5000.000"
	)
	wsID := types.WorkspaceID("ws-life")

	// --- repos & registry --------------------------------------------------
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: wsID, Name: "Life"},
		FieldSchema: &config.FieldSchema{
			Statuses:     []config.StatusDef{{ID: "open", Name: "Open"}},
			TicketConfig: config.TicketConfig{DefaultStatusID: "open"},
		},
		SlackChannelID: types.SlackChannelID(channel),
	})
	hist := newFakeHistory()

	// --- LLM mock sequenced by call count ---------------------------------
	//
	// The mock simulates a real LLM session by accumulating an internal
	// *gollem.History per session. AppendHistory merges agent-driven writes
	// (user inputs, assistant text responses) into internal, and HistoryFunc
	// returns the running internal history so the agent's saveHistoryToRepo
	// pass persists every turn's plan JSON into the configured repo. Without
	// this, the second planner turn would not see the first turn's plan
	// message and the executor's ask -> submit -> resume state machine would
	// break.
	var llmCalls int32
	// llmPlan calls Session.Generate exactly once per planner turn (the
	// schema-constrained JSON output is the entire response — no tool loop),
	// so the lifecycle traverses two turns total: turn 1 posts the ask, turn
	// 2 (resumed by Submit) completes triage.
	makeResp := func(t *testing.T, n int32) *gollem.Response {
		t.Helper()
		switch n {
		case 1:
			return &gollem.Response{Texts: []string{askPlanJSON}}
		case 2:
			return &gollem.Response{Texts: []string{completePlanJSON}}
		default:
			t.Fatalf("LLM should not be called more than 2 times, got %d", n)
			return nil
		}
	}

	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, opts ...gollem.SessionOption) (gollem.Session, error) {
			cfg := gollem.NewSessionConfig(opts...)
			internal := &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}
			if h := cfg.History(); h != nil {
				internal.Messages = append(internal.Messages, h.Messages...)
			}
			return &mock.SessionMock{
				GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
					n := atomic.AddInt32(&llmCalls, 1)
					return makeResp(t, n), nil
				},
				HistoryFunc: func() (*gollem.History, error) {
					return internal.Clone(), nil
				},
				AppendHistoryFunc: func(h *gollem.History) error {
					if h != nil {
						internal.Messages = append(internal.Messages, h.Messages...)
					}
					return nil
				},
			}, nil
		},
	}

	// --- usecases ---------------------------------------------------------
	userSlack := &fakeUserSlack{}
	triageSlack := &fakeTriageSlack{}
	catalog := tool.NewCatalog(nil, repo.ToolSettings())
	promptUC := prompt.New(repo.Prompt())
	// Lifecycle test asserts the planner converges to Triaged=true; opt the
	// workspace into auto-finalise so we exercise the legacy fast path
	// without the reporter-review hop.
	exec := triage.NewPlanExecutor(repo, hist, llm, triageSlack, catalog, promptUC,
		&fakeWorkspaceLookup{auto: map[types.WorkspaceID]bool{wsID: true}},
		triage.Config{IterationCap: 5})
	triageUC := triage.NewUseCase(exec, &fakeResolver{ws: wsID, channel: channel})

	slackUC := usecase.NewSlackUseCase(repo, registry, userSlack, "https://shepherd.example.com", llm, hist, nil)
	slackUC.SetTriageTrigger(triageUC)

	ctx := context.Background()

	// === Step 1: ticket creation triggers triage ==========================
	gt.NoError(t, slackUC.HandleNewMessage(ctx, channel, reporter, "Login broken", threadTS))
	async.Wait()

	tickets := gt.R1(repo.Ticket().List(ctx, wsID, nil)).NoError(t)
	gt.A(t, tickets).Length(1)
	ticket := tickets[0]
	gt.S(t, ticket.Title).Equal("Login broken")
	gt.False(t, ticket.Triaged)

	// SlackUseCase posted the ticket-created reply.
	gt.A(t, userSlack.ticketCreated).Length(1)
	gt.S(t, userSlack.ticketCreated[0].channelID).Equal(channel)

	// First planner turn ran and produced propose_ask in plan history.
	plan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanAsk)

	// Triage posted exactly one Slack message — the ask form — in the
	// ticket thread.
	gt.A(t, triageSlack.posts).Length(1)
	gt.S(t, triageSlack.posts[0].channel).Equal(channel)
	gt.S(t, triageSlack.posts[0].threadTS).Equal(threadTS)
	askMessageTS := "ts-" + threadTS // fakeTriageSlack synthesises ts-<threadTS> for posts

	// We are now in the waiting-for-submit state.
	waiting := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.True(t, waiting)

	// === Step 2: reporter submits answers =================================
	gt.NoError(t, triageUC.HandleSubmit(ctx, triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: channel, MessageTS: askMessageTS,
		State: stateFor("c1", ""),
	}))
	async.Wait()

	// Slack: the ask message was updated (received notice) before the
	// completion message was posted. We don't pin the exact count of
	// updates because the executor may emit additional progress mutations
	// in future, but the first update must target the ask message.
	gt.True(t, len(triageSlack.updates) >= 1)
	gt.S(t, triageSlack.updates[0].messageTS).Equal(askMessageTS)

	// History: the formatted answers landed as a user-role text message
	// after the propose_ask tool call.
	hh := gt.R1(hist.Load(ctx, triage.PlanSessionIDForTest(wsID, ticket.ID))).NoError(t)
	gt.NotNil(t, hh)
	var userTexts []string
	for _, m := range hh.Messages {
		if m.Role != gollem.RoleUser {
			continue
		}
		for _, c := range m.Contents {
			if c.Type == gollem.MessageContentTypeText {
				tc, err := c.GetTextContent()
				if err == nil {
					userTexts = append(userTexts, tc.Text)
				}
			}
		}
	}
	gt.True(t, len(userTexts) >= 1)
	joined := strings.Join(userTexts, "\n")
	gt.True(t, strings.Contains(joined, "What is the scope of impact?"))
	gt.True(t, strings.Contains(joined, "Production"))

	// === Step 3: planner resumed and completed triage =====================
	got := gt.R1(repo.Ticket().Get(ctx, wsID, ticket.ID)).NoError(t)
	gt.True(t, got.Triaged)
	gt.A(t, got.AssigneeIDs).Length(1)
	gt.Equal(t, got.AssigneeIDs[0], types.SlackUserID("U123"))

	// Triage posted the hand-off summary in the ticket thread (in addition
	// to the original ask form).
	gt.True(t, len(triageSlack.posts) >= 2)
	last := triageSlack.posts[len(triageSlack.posts)-1]
	gt.S(t, last.channel).Equal(channel)
	gt.S(t, last.threadTS).Equal(threadTS)
	gt.True(t, len(last.blocks) > 0)

	// LLM was driven exactly twice — turn 1 (ask) + turn 2 (complete).
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(2)

	// Latest plan in history is now propose_complete, confirming the
	// resumed turn was persisted.
	finalPlan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.Equal(t, finalPlan.Kind, types.PlanComplete)

	// A submit on the now-completed ticket should be a no-op (form
	// invalidated), proving the Triaged guard holds across re-deliveries.
	prevUpdates := len(triageSlack.updates)
	gt.NoError(t, triageUC.HandleSubmit(ctx, triage.Submission{
		TicketID:  ticket.ID,
		ChannelID: channel, MessageTS: askMessageTS,
		State: stateFor("c1", ""),
	}))
	async.Wait()
	gt.N(t, len(triageSlack.updates)).Equal(prevUpdates + 1) // one invalidation update, no new posts
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(2)        // LLM was NOT invoked again
}

// autoFillSchema returns a FieldSchema with one auto_fill select field
// (severity, required) and one auto_fill multi-select field (tags, optional).
// The shape mirrors the on-disk config a workspace would declare; the
// lifecycle tests use it to drive the planner's auto_fill briefing + the
// validator + the suggested-fields rendering on the review message.
func autoFillSchema() *config.FieldSchema {
	return &config.FieldSchema{
		Statuses: []config.StatusDef{{ID: types.StatusID("open"), Name: "Open"}},
		TicketConfig: config.TicketConfig{
			DefaultStatusID: types.StatusID("open"),
		},
		Fields: []config.FieldDefinition{
			{
				ID: "severity", Name: "Severity",
				Type:     types.FieldTypeSelect,
				Required: true, AutoFill: true,
				Options: []config.FieldOption{
					{ID: "p0", Name: "Sev 0 — outage"},
					{ID: "p1", Name: "Sev 1 — major"},
				},
			},
			{
				ID: "tags", Name: "Tags",
				Type: types.FieldTypeMultiSelect, AutoFill: true,
				Options: []config.FieldOption{
					{ID: "frontend", Name: "Frontend"},
					{ID: "backend", Name: "Backend"},
				},
			},
		},
	}
}

const completePlanInvalidAutoFillJSON = `{
  "kind": "complete",
  "message": "Done",
  "complete": {
    "summary": "Investigation done",
    "assignee": {
      "kind": "assigned",
      "user_ids": ["U123"],
      "reasoning": "owner"
    },
    "suggested_fields": {
      "severity": "made-up-severity",
      "tags": ["frontend"]
    }
  }
}`

const completePlanValidAutoFillJSON = `{
  "kind": "complete",
  "message": "Done",
  "complete": {
    "summary": "Investigation done",
    "assignee": {
      "kind": "assigned",
      "user_ids": ["U123"],
      "reasoning": "owner"
    },
    "suggested_fields": {
      "severity": "p0",
      "tags": ["frontend", "backend"]
    }
  }
}`

// blockJSON renders a block to JSON so tests can substring-check the rendered
// text without depending on the slack-go block type tree.
func blockJSON(t *testing.T, blocks []slackgo.Block) string {
	t.Helper()
	out := strings.Builder{}
	for _, b := range blocks {
		raw, err := json.Marshal(b)
		gt.NoError(t, err)
		out.Write(raw)
		out.WriteByte('\n')
	}
	return out.String()
}

// TestLifecycle_AutoFill_RetriesThenPostsReviewWithSuggestedFields drives
// the full planner loop with a workspace that has auto_fill custom fields.
// The first LLM response is rejected by validatePlanAutoFill (option id
// outside the allow-list); the executor must:
//
//   - post the i18n "retrying" notice on the ticket thread,
//   - feed the verbatim error back as the next user turn (history grows),
//   - re-invoke the planner exactly once,
//   - and on the valid response, park on the reporter-review buttons with a
//     "Suggested field values" section whose labels come from the schema.
func TestLifecycle_AutoFill_RetriesThenPostsReviewWithSuggestedFields(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	hist := newFakeHistory()
	slack := &fakeTriageSlack{}

	var llmCalls int32
	scripted := []string{
		completePlanInvalidAutoFillJSON,
		completePlanValidAutoFillJSON,
	}
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			internal := &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}
			return &mock.SessionMock{
				GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
					n := atomic.AddInt32(&llmCalls, 1)
					if int(n) > len(scripted) {
						t.Fatalf("Generate called %d times; script has %d entries", n, len(scripted))
					}
					return &gollem.Response{Texts: []string{scripted[n-1]}}, nil
				},
				HistoryFunc:       func() (*gollem.History, error) { return internal.Clone(), nil },
				AppendHistoryFunc: func(h *gollem.History) error {
					if h != nil {
						internal.Messages = append(internal.Messages, h.Messages...)
					}
					return nil
				},
			}, nil
		},
	}

	catalog := tool.NewCatalog(nil, repo.ToolSettings())
	promptUC := prompt.New(repo.Prompt())
	lookup := &fakeWorkspaceLookup{
		auto:    map[types.WorkspaceID]bool{}, // auto=false → enterReview path
		schemas: map[types.WorkspaceID]*config.FieldSchema{tWS: autoFillSchema()},
	}
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, catalog, promptUC, lookup,
		triage.Config{IterationCap: 5, PlanRetryCap: 2})
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))

	// LLM was called exactly twice: invalid then valid.
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(2)

	// Slack: exactly one retry notice on the thread.
	gt.A(t, slack.replies).Length(1)
	gt.S(t, slack.replies[0].channel).Equal(tChannel)
	gt.S(t, slack.replies[0].threadTS).Equal(tThread)
	gt.True(t, strings.Contains(slack.replies[0].text, "Re-running") ||
		strings.Contains(slack.replies[0].text, "再実行"))

	// Slack: one post — the review message (auto=false so we hit enterReview).
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)

	// The review blocks must surface the suggested-field section with
	// schema-resolved labels (not raw option ids), proving the planner's
	// auto-fill values reach the reporter for confirmation.
	rendered := blockJSON(t, slack.posts[0].blocks)
	for _, want := range []string{
		"Severity",         // field name from schema
		"Sev 0 — outage",   // resolved option label for "p0"
		"Tags",             // multi-select field name
		"Frontend",         // resolved option label
		"Backend",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("review blocks missing %q\n---\n%s", want, rendered)
		}
	}

	// Ticket must NOT be triaged — auto=false parks on the review buttons.
	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)

	// The latest persisted plan must carry the *valid* auto_fill values, not
	// the invalid first response. This proves validation rejected attempt #1
	// before it landed in the canonical TriagePlan.
	plan := gt.R1(triage.LoadLatestTriagePlanForTest(context.Background(), hist, tWS, ticket.ID)).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanComplete)
	gt.NotNil(t, plan.Complete)
	gt.Equal(t, plan.Complete.SuggestedFields["severity"], "p0")
}

// TestLifecycle_AutoFill_RetryCapExhausted_PostsFailureMessage verifies the
// error path: with PlanRetryCap=0 a single invalid response is enough to
// surface as a failure-recovery post (the standard retry-button message),
// rather than silently looping.
func TestLifecycle_AutoFill_RetryCapExhausted_PostsFailureMessage(t *testing.T) {
	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })
	hist := newFakeHistory()
	slack := &fakeTriageSlack{}

	var llmCalls int32
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			internal := &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}
			return &mock.SessionMock{
				GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
					atomic.AddInt32(&llmCalls, 1)
					return &gollem.Response{Texts: []string{completePlanInvalidAutoFillJSON}}, nil
				},
				HistoryFunc:       func() (*gollem.History, error) { return internal.Clone(), nil },
				AppendHistoryFunc: func(h *gollem.History) error {
					if h != nil {
						internal.Messages = append(internal.Messages, h.Messages...)
					}
					return nil
				},
			}, nil
		},
	}

	catalog := tool.NewCatalog(nil, repo.ToolSettings())
	promptUC := prompt.New(repo.Prompt())
	lookup := &fakeWorkspaceLookup{
		auto:    map[types.WorkspaceID]bool{},
		schemas: map[types.WorkspaceID]*config.FieldSchema{tWS: autoFillSchema()},
	}
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, catalog, promptUC, lookup,
		triage.Config{IterationCap: 5, PlanRetryCap: 0})
	ticket := mustCreateTicket(t, repo, false)

	// On a validation failure with retries exhausted, run() bubbles the error
	// up *and* the deferred handler posts the recovery message. We assert
	// both: the error surfaces (so caller logging still works) and the
	// reporter sees the standard retry-button message in the thread.
	err := exec.RunForTest(context.Background(), tWS, ticket.ID)
	gt.Error(t, err)

	// LLM was called exactly once — the retry cap is 0 so no second attempt.
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(1)

	// No retry notice on the thread (cap=0 means we never reach the notify step).
	gt.A(t, slack.replies).Length(0)

	// Failure-recovery message was posted to the thread.
	gt.A(t, slack.posts).Length(1)

	// Ticket stays un-triaged.
	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)
}
