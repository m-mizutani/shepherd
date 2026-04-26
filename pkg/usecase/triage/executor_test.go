package triage_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
)

func TestExecutorRun_AlreadyTriaged_NoLLMCall(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM must not be invoked when ticket.Triaged is true")
			return nil, nil
		},
	}
	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, true)

	gt.NoError(t, exec.Run(context.Background(), tWS, ticket.ID))
	gt.A(t, slack.posts).Length(0)
	gt.A(t, slack.updates).Length(0)
}

func TestExecutorRun_WaitingForSubmit_NoLLMCall(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM must not be invoked while waiting on a Slack submit")
			return nil, nil
		},
	}
	_, exec, repo, hist, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)
	seedAskHistory(t, hist, ticket.ID)

	gt.NoError(t, exec.Run(context.Background(), tWS, ticket.ID))
	gt.A(t, slack.posts).Length(0)
	gt.A(t, slack.updates).Length(0)
}

func TestExecutorRun_IterationCapExceeded_FinalizesAsAborted(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM must not be invoked once the cap is exhausted")
			return nil, nil
		},
	}
	_, exec, repo, hist, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)

	// Pre-populate the plan history with `cap` propose_* tool calls so the
	// next Run sees count >= cap and short-circuits to abort. cap is 5
	// per newRig.
	h := &gollem.History{Version: gollem.HistoryVersion}
	for range 5 {
		h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeInvestigateToolName, investigateArgs()))
		// Pair each tool call with a user-role message so IsWaitingUserSubmit
		// stays false (no trailing propose_ask).
		h.Messages = append(h.Messages, mustUserTextMessage(t, "result"))
	}
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionID(tWS, ticket.ID), h))

	gt.NoError(t, exec.Run(context.Background(), tWS, ticket.ID))

	// Ticket should now be Triaged via the abort path.
	got, err := repo.Ticket().Get(context.Background(), tWS, ticket.ID)
	gt.NoError(t, err)
	gt.True(t, got.Triaged)

	// Abort posts a single thread message announcing the abort reason.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)
}

func TestExecutorRun_LLMProposesComplete_FinalizesTriage(t *testing.T) {
	var generateCalls int32
	args := completeArgs()
	session := &mock.SessionMock{
		GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			n := atomic.AddInt32(&generateCalls, 1)
			if n == 1 {
				return &gollem.Response{
					FunctionCalls: []*gollem.FunctionCall{
						{
							ID:        "fc-1",
							Name:      triage.ProposeCompleteToolName,
							Arguments: args,
						},
					},
				}, nil
			}
			// Subsequent calls: return an empty response so the agent loop
			// exits naturally.
			return &gollem.Response{}, nil
		},
		HistoryFunc: func() (*gollem.History, error) {
			return &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}, nil
		},
		AppendHistoryFunc: func(_ *gollem.History) error { return nil },
	}
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, exec.Run(context.Background(), tWS, ticket.ID))

	// Persistence: Triaged flag flipped, assignee from completeArgs persisted.
	got, err := repo.Ticket().Get(context.Background(), tWS, ticket.ID)
	gt.NoError(t, err)
	gt.True(t, got.Triaged)
	gt.Equal(t, got.AssigneeID, types.SlackUserID("U123"))

	// Slack: hand-off summary posted in the ticket thread.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)

	// LLM was invoked at least once (at most twice — first round = the tool
	// call, second round = empty response that ends the loop).
	gt.True(t, atomic.LoadInt32(&generateCalls) >= 1)
}

func TestExecutorRun_LLMProposesAsk_PostsFormAndPauses(t *testing.T) {
	args := askArgs()
	var generateCalls int32
	session := &mock.SessionMock{
		GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			n := atomic.AddInt32(&generateCalls, 1)
			if n == 1 {
				return &gollem.Response{
					FunctionCalls: []*gollem.FunctionCall{
						{ID: "fc-ask", Name: triage.ProposeAskToolName, Arguments: args},
					},
				}, nil
			}
			return &gollem.Response{}, nil
		},
		HistoryFunc: func() (*gollem.History, error) {
			return &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}, nil
		},
		AppendHistoryFunc: func(_ *gollem.History) error { return nil },
	}
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, exec.Run(context.Background(), tWS, ticket.ID))

	// Ticket must NOT be triaged yet — Ask just pauses the loop.
	got, err := repo.Ticket().Get(context.Background(), tWS, ticket.ID)
	gt.NoError(t, err)
	gt.False(t, got.Triaged)

	// Slack: the question form was posted in the thread.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)
}
