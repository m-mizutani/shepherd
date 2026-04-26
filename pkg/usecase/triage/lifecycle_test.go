package triage_test

import (
	"context"
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
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
)

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
	// *gollem.History per session: each Generate that returns FunctionCalls
	// also appends a corresponding assistant ToolCall message, and each
	// AppendHistory call merges the input message into internal. HistoryFunc
	// returns the running internal history so the agent's saveHistoryToRepo
	// pass writes the propose_* tool call into the configured history repo.
	// Without this, the second planner turn would not see the first turn's
	// tool call and the executor's propose_ask state machine would break.
	askPayload := askArgs()
	completePayload := completeArgs()
	var llmCalls int32
	makeResp := func(t *testing.T, n int32) *gollem.Response {
		t.Helper()
		switch n {
		case 1:
			return &gollem.Response{
				FunctionCalls: []*gollem.FunctionCall{
					{ID: "fc-ask", Name: triage.ProposeAskToolName, Arguments: askPayload},
				},
			}
		case 3:
			return &gollem.Response{
				FunctionCalls: []*gollem.FunctionCall{
					{ID: "fc-comp", Name: triage.ProposeCompleteToolName, Arguments: completePayload},
				},
			}
		case 2, 4:
			// Loop tail: empty response so the agent loop exits after the
			// preceding tool call.
			return &gollem.Response{}
		default:
			t.Fatalf("LLM should not be called more than 4 times, got %d", n)
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
					resp := makeResp(t, n)
					if resp != nil && len(resp.FunctionCalls) > 0 {
						contents := make([]gollem.MessageContent, 0, len(resp.FunctionCalls))
						for _, fc := range resp.FunctionCalls {
							c, err := gollem.NewToolCallContent(fc.ID, fc.Name, fc.Arguments)
							if err != nil {
								return nil, err
							}
							contents = append(contents, c)
						}
						internal.Messages = append(internal.Messages, gollem.Message{
							Role:     gollem.RoleAssistant,
							Contents: contents,
						})
					}
					return resp, nil
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
	exec := triage.NewPlanExecutor(repo, hist, llm, triageSlack, catalog, triage.Config{IterationCap: 5})
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
	plan := gt.R1(triage.LoadLatestTriagePlan(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanAsk)

	// Triage posted exactly one Slack message — the ask form — in the
	// ticket thread.
	gt.A(t, triageSlack.posts).Length(1)
	gt.S(t, triageSlack.posts[0].channel).Equal(channel)
	gt.S(t, triageSlack.posts[0].threadTS).Equal(threadTS)
	askMessageTS := "ts-" + threadTS // fakeTriageSlack synthesises ts-<threadTS> for posts

	// We are now in the waiting-for-submit state.
	waiting := gt.R1(triage.IsWaitingUserSubmit(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.True(t, waiting)

	// === Step 2: reporter submits answers =================================
	gt.NoError(t, triageUC.HandleSubmit(ctx, triage.Submission{
		WorkspaceID: wsID, TicketID: ticket.ID,
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
	hh := gt.R1(hist.Load(ctx, triage.PlanSessionID(wsID, ticket.ID))).NoError(t)
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
	gt.True(t, strings.Contains(joined, "影響範囲は？"))
	gt.True(t, strings.Contains(joined, "本番"))

	// === Step 3: planner resumed and completed triage =====================
	got := gt.R1(repo.Ticket().Get(ctx, wsID, ticket.ID)).NoError(t)
	gt.True(t, got.Triaged)
	gt.Equal(t, got.AssigneeID, types.SlackUserID("U123"))

	// Triage posted the hand-off summary in the ticket thread (in addition
	// to the original ask form).
	gt.True(t, len(triageSlack.posts) >= 2)
	last := triageSlack.posts[len(triageSlack.posts)-1]
	gt.S(t, last.channel).Equal(channel)
	gt.S(t, last.threadTS).Equal(threadTS)
	gt.True(t, len(last.blocks) > 0)

	// LLM was driven exactly 4 times: ask + tail, complete + tail. Going
	// over 4 (or under 3) means the loop misbehaved.
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(4)

	// Latest plan in history is now propose_complete, confirming the
	// resumed turn was persisted.
	finalPlan := gt.R1(triage.LoadLatestTriagePlan(ctx, hist, wsID, ticket.ID)).NoError(t)
	gt.Equal(t, finalPlan.Kind, types.PlanComplete)

	// A submit on the now-completed ticket should be a no-op (form
	// invalidated), proving the Triaged guard holds across re-deliveries.
	prevUpdates := len(triageSlack.updates)
	gt.NoError(t, triageUC.HandleSubmit(ctx, triage.Submission{
		WorkspaceID: wsID, TicketID: ticket.ID,
		ChannelID: channel, MessageTS: askMessageTS,
		State: stateFor("c1", ""),
	}))
	async.Wait()
	gt.N(t, len(triageSlack.updates)).Equal(prevUpdates + 1) // one invalidation update, no new posts
	gt.N(t, int(atomic.LoadInt32(&llmCalls))).Equal(4)        // LLM was NOT invoked again
}
