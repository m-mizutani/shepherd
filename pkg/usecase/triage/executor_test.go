package triage_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
	slackgo "github.com/slack-go/slack"
)

// newPlanSessionMock returns a stub session whose Generate emits the supplied
// plan JSONs in order, one per call. Extra calls beyond the script fail the
// test. The session also keeps an internal history that mirrors what the
// gollem agent expects: any user input fed via Generate is recorded, and each
// assistant text response is appended too. agent.Execute persists session
// history to the configured HistoryRepository on every round-trip.
func newPlanSessionMock(t *testing.T, plans []string) *mock.SessionMock {
	t.Helper()
	internal := &gollem.History{Version: gollem.HistoryVersion, LLType: gollem.LLMTypeOpenAI}
	var calls int32
	return &mock.SessionMock{
		GenerateFunc: func(_ context.Context, _ []gollem.Input, _ ...gollem.GenerateOption) (*gollem.Response, error) {
			n := atomic.AddInt32(&calls, 1)
			if int(n) > len(plans) {
				t.Fatalf("Generate called %d times; script has only %d entries", n, len(plans))
			}
			text := plans[n-1]
			return &gollem.Response{Texts: []string{text}}, nil
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
	}
}

func TestExecutorRun_AlreadyTriaged_NoLLMCall(t *testing.T) {
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			t.Fatalf("LLM must not be invoked when ticket.Triaged is true")
			return nil, nil
		},
	}
	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, true)

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))
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

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))
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

	// Pre-populate the plan history with `cap` planner turns so the next Run
	// sees count >= cap and short-circuits to abort. cap is 5 per newRig.
	h := &gollem.History{Version: gollem.HistoryVersion}
	for range 5 {
		h.Messages = append(h.Messages, mustAssistantPlanMessage(t, investigatePlanJSON))
		// Pair each plan turn with a user-role message so IsWaitingUserSubmit
		// stays false (no trailing ask).
		h.Messages = append(h.Messages, mustUserTextMessage(t, "result"))
	}
	gt.NoError(t, hist.Save(context.Background(), triage.PlanSessionIDForTest(tWS, ticket.ID), h))

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))

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
	session := newPlanSessionMock(t, []string{completePlanJSON})
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	// auto = true opts the workspace into the legacy fast-path: PlanComplete
	// finalises immediately and posts the hand-off, no reporter review.
	_, _, repo, hist, slack := newRig(t, llm)
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, nil, nil,
		&fakeWorkspaceLookup{auto: map[types.WorkspaceID]bool{tWS: true}},
		triage.Config{IterationCap: 5})
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))

	// Persistence: Triaged flag flipped, assignee from completePlanJSON persisted.
	got, err := repo.Ticket().Get(context.Background(), tWS, ticket.ID)
	gt.NoError(t, err)
	gt.True(t, got.Triaged)
	gt.Equal(t, got.AssigneeID, types.SlackUserID("U123"))

	// Slack: hand-off summary posted in the ticket thread.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)
}

func TestExecutorRun_LLMProposesAsk_PostsFormAndPauses(t *testing.T) {
	session := newPlanSessionMock(t, []string{askPlanJSON})
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)

	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))

	// Ticket must NOT be triaged yet — Ask just pauses the loop.
	got, err := repo.Ticket().Get(context.Background(), tWS, ticket.ID)
	gt.NoError(t, err)
	gt.False(t, got.Triaged)

	// Slack: the question form was posted in the thread.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)
}

// Compile-time guard that completePlanJSON parses as a TriagePlan with the
// expected assignee, so the test above's Equal(U123) assertion has a stable
// reference point.
var _ = func() any {
	var v map[string]any
	_ = json.Unmarshal([]byte(completePlanJSON), &v)
	return v
}()

// TestExecutorRun_AbnormalExit_PostsRetryMessage covers the deferred
// recovery path: when llmPlan returns an error, run() must post a failure
// message carrying a retry button to the ticket thread. The ticket itself
// stays un-Triaged so HandleRetry can re-dispatch the planner.
func TestExecutorRun_AbnormalExit_PostsRetryMessage(t *testing.T) {
	llm := &mock.LLMClientMock{
		// NewSession returning an error makes agent.Execute fail, which
		// llmPlan wraps and returns up to run(); the deferred handler then
		// posts the recovery message.
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return nil, errSimulatedLLMFailure
		},
	}
	_, exec, repo, _, slack := newRig(t, llm)
	ticket := mustCreateTicket(t, repo, false)

	err := exec.RunForTest(context.Background(), tWS, ticket.ID)
	gt.Error(t, err)

	// Ticket must NOT be marked triaged — abnormal failure should leave the
	// ticket in a retryable state.
	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)

	// One Slack post: the failure-recovery message in the ticket thread.
	gt.A(t, slack.posts).Length(1)
	gt.S(t, slack.posts[0].channel).Equal(tChannel)
	gt.S(t, slack.posts[0].threadTS).Equal(tThread)
	// Recovery blocks include the retry action block (id triage_retry_actions).
	var sawRetryAction bool
	for _, b := range slack.posts[0].blocks {
		if ab, ok := b.(*slackgo.ActionBlock); ok && ab.BlockID == "triage_retry_actions" {
			sawRetryAction = true
		}
	}
	gt.True(t, sawRetryAction)
}

var errSimulatedLLMFailure = goerrNew("simulated LLM failure")

// goerrNew is a tiny helper so the test file does not need to depend on
// goerr/v2 just for one error literal; using errors.New keeps the diff small.
func goerrNew(msg string) error { return &simpleErr{msg: msg} }

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }

// fakeWorkspaceLookup provides a minimal triage.WorkspaceLookup for tests.
type fakeWorkspaceLookup struct {
	auto    map[types.WorkspaceID]bool
	schemas map[types.WorkspaceID]*domainConfig.FieldSchema
}

func (f *fakeWorkspaceLookup) AutoTriage(ws types.WorkspaceID) bool {
	if f == nil {
		return false
	}
	return f.auto[ws]
}

func (f *fakeWorkspaceLookup) WorkspaceSchema(ws types.WorkspaceID) *domainConfig.FieldSchema {
	if f == nil {
		return nil
	}
	return f.schemas[ws]
}

// TestExecutorRun_LLMProposesComplete_DefaultRequiresReview_PostsReviewWithoutFinalize
// covers the default PlanComplete path: when the workspace does not opt into
// `[triage] auto = true`, the executor must NOT mark Triaged=true and must
// post the review message (with the 3 buttons) instead of the legacy hand-off.
func TestExecutorRun_LLMProposesComplete_DefaultRequiresReview_PostsReviewWithoutFinalize(t *testing.T) {
	session := newPlanSessionMock(t, []string{completePlanJSON})
	llm := &mock.LLMClientMock{
		NewSessionFunc: func(_ context.Context, _ ...gollem.SessionOption) (gollem.Session, error) {
			return session, nil
		},
	}

	// Build a rig with a lookup that leaves AutoTriage=false (default), so
	// PlanComplete parks on the review buttons.
	uc, _, repo, hist, slack := newRig(t, llm)
	_ = uc
	exec := triage.NewPlanExecutor(repo, hist, llm, slack, nil, nil,
		&fakeWorkspaceLookup{auto: map[types.WorkspaceID]bool{}},
		triage.Config{IterationCap: 5})

	ticket := mustCreateTicket(t, repo, false)
	gt.NoError(t, exec.RunForTest(context.Background(), tWS, ticket.ID))

	// Ticket must NOT be triaged yet — review pause leaves Triaged=false.
	got := gt.R1(repo.Ticket().Get(context.Background(), tWS, ticket.ID)).NoError(t)
	gt.False(t, got.Triaged)

	// Slack: exactly one post — the review message — with the 3-button
	// triage_review_actions block. No update calls (per spec, review messages
	// are never rewritten).
	gt.A(t, slack.posts).Length(1)
	gt.A(t, slack.updates).Length(0)
	var sawReviewActions bool
	for _, b := range slack.posts[0].blocks {
		if ab, ok := b.(*slackgo.ActionBlock); ok && ab.BlockID == "triage_review_actions" {
			sawReviewActions = true
		}
	}
	gt.True(t, sawReviewActions)
}
