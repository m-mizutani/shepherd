package triage_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	"github.com/m-mizutani/shepherd/pkg/tool"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	"github.com/m-mizutani/shepherd/pkg/utils/async"
	slackgo "github.com/slack-go/slack"
)

// fakeTriageSlack records every Slack call the triage executor makes through
// SlackTriageClient so tests can assert exact ordering / payloads.
type fakeTriageSlack struct {
	mu       sync.Mutex
	posts    []postCall
	updates  []updateCall
	replies  []replyCall
	postErr  error
	updateErr error
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
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, catalog, triage.Config{IterationCap: 5})
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
	h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeAskToolName, askArgs()))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionID(tWS, ticketID), h))
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
		WorkspaceID: tWS, TicketID: ticket.ID,
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
		WorkspaceID: tWS, TicketID: ticket.ID,
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
	h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeInvestigateToolName, investigateArgs()))
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionID(tWS, ticket.ID), h))

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		WorkspaceID: tWS, TicketID: ticket.ID,
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
		mustToolCallMessage(t, triage.ProposeAskToolName, askArgs()),
		mustUserTextMessage(t, "previous answer"),
	)
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionID(tWS, ticket.ID), h))

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		WorkspaceID: tWS, TicketID: ticket.ID,
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
		WorkspaceID: tWS, TicketID: ticket.ID,
		ChannelID: tChannel, MessageTS: "msg-5",
		State: stateFor("", ""),
	}))

	gt.A(t, slack.updates).Length(1)
	// History must NOT have been appended with answers.
	hh, _ := hist.Load(context.Background(), triage.PlanSessionID(tWS, ticket.ID))
	gt.NotNil(t, hh)
	for _, msg := range hh.Messages {
		if msg.Role == gollem.RoleUser {
			t.Fatalf("validation error path must not append a user-role message to history")
		}
	}
}

func TestHandleSubmit_HappyPath_AppendsAnswerAndAcksMessage(t *testing.T) {
	uc, _, repo, hist, slack := newRig(t, nil)
	ticket := mustCreateTicket(t, repo, false)
	seedAskHistory(t, hist, ticket.ID)

	gt.NoError(t, uc.HandleSubmit(context.Background(), triage.Submission{
		WorkspaceID: tWS, TicketID: ticket.ID,
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
	hh, _ := hist.Load(context.Background(), triage.PlanSessionID(tWS, ticket.ID))
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
	gt.True(t, strings.Contains(userMessages[0], "影響範囲は？"))
	gt.True(t, strings.Contains(userMessages[0], "本番"))
}

func TestWorkspaceForChannel(t *testing.T) {
	uc, _, _, _, _ := newRig(t, nil)
	ws, ok := uc.WorkspaceForChannel(context.Background(), tChannel)
	gt.True(t, ok)
	gt.Equal(t, ws, tWS)

	_, ok = uc.WorkspaceForChannel(context.Background(), "C-unknown")
	gt.False(t, ok)
}

func TestTicketByID_NotFound_ReturnsNil(t *testing.T) {
	uc, _, _, _, _ := newRig(t, nil)
	tk, err := uc.TicketByID(context.Background(), tWS, types.TicketID("missing"))
	gt.NoError(t, err)
	gt.Nil(t, tk)
}
