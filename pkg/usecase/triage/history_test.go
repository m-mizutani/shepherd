package triage_test

import (
	"context"
	"sync"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
)

type fakeHistoryRepo struct {
	mu    sync.Mutex
	store map[string]*gollem.History
}

func newFakeHistory() *fakeHistoryRepo {
	return &fakeHistoryRepo{store: make(map[string]*gollem.History)}
}

func (r *fakeHistoryRepo) Load(ctx context.Context, sessionID string) (*gollem.History, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.store[sessionID]; ok {
		return h.Clone(), nil
	}
	return nil, nil
}

func (r *fakeHistoryRepo) Save(ctx context.Context, sessionID string, h *gollem.History) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[sessionID] = h.Clone()
	return nil
}

// mustAssistantPlanMessage builds an assistant message carrying the given
// raw JSON payload — exactly the shape agent.Execute persists when the
// session runs under WithContentType(JSON) + WithResponseSchema. Used to
// pre-seed the planner's history in tests.
func mustAssistantPlanMessage(t *testing.T, planJSON string) gollem.Message {
	t.Helper()
	c := gt.R1(gollem.NewTextContent(planJSON)).NoError(t)
	return gollem.Message{
		Role:     gollem.RoleAssistant,
		Contents: []gollem.MessageContent{c},
	}
}

func mustUserTextMessage(t *testing.T, text string) gollem.Message {
	t.Helper()
	c := gt.R1(gollem.NewTextContent(text)).NoError(t)
	return gollem.Message{
		Role:     gollem.RoleUser,
		Contents: []gollem.MessageContent{c},
	}
}

const investigatePlanJSON = `{
  "kind": "investigate",
  "message": "Investigating now",
  "investigate": {
    "subtasks": [
      {
        "id": "st1",
        "request": "Collect related Slack posts",
        "acceptance_criteria": ["return at least 3 messages or explicit none"],
        "allowed_tools": ["slack_search_messages"]
      }
    ]
  }
}`

const askPlanJSON = `{
  "kind": "ask",
  "message": "Need clarification",
  "ask": {
    "title": "Details",
    "questions": [
      {
        "id": "q1",
        "label": "What is the scope of impact?",
        "choices": [
          {"id": "c1", "label": "Production"},
          {"id": "c2", "label": "Staging"}
        ]
      }
    ]
  }
}`

const completePlanJSON = `{
  "kind": "complete",
  "message": "Done",
  "summary": "Investigation done",
  "complete": {
    "summary": "Investigation done",
    "assignee": {
      "kind": "assigned",
      "user_id": "U123",
      "reasoning": "owner of the affected service"
    }
  }
}`

func TestPlanSessionID(t *testing.T) {
	got := triage.PlanSessionIDForTest(types.WorkspaceID("ws1"), types.TicketID("t1"))
	gt.S(t, got).Equal("ws1/t1/plan")
}

func TestSubtaskSessionID(t *testing.T) {
	got := triage.SubtaskSessionIDForTest("ws1", "t1", "stA")
	gt.S(t, got).Equal("ws1/t1/sub/stA")
}

func TestAppendUserMessage(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	gt.NoError(t, triage.AppendUserMessageForTest(ctx, repo, "ws", "tk", "hello"))

	h := gt.R1(repo.Load(ctx, "ws/tk/plan")).NoError(t)
	gt.NotNil(t, h)
	gt.N(t, len(h.Messages)).Equal(1)
	gt.Equal(t, h.Messages[0].Role, gollem.RoleUser)

	tc := gt.R1(h.Messages[0].Contents[0].GetTextContent()).NoError(t)
	gt.S(t, tc.Text).Equal("hello")
}

func TestAppendUserMessage_AppendsToExisting(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	gt.NoError(t, triage.AppendUserMessageForTest(ctx, repo, "ws", "tk", "first"))
	gt.NoError(t, triage.AppendUserMessageForTest(ctx, repo, "ws", "tk", "second"))

	h := gt.R1(repo.Load(ctx, "ws/tk/plan")).NoError(t)
	gt.N(t, len(h.Messages)).Equal(2)
}

func TestLoadLatestTriagePlan_Empty(t *testing.T) {
	repo := newFakeHistory()
	plan := gt.R1(triage.LoadLatestTriagePlanForTest(context.Background(), repo, "ws", "tk")).NoError(t)
	gt.Nil(t, plan)
}

func TestLoadLatestTriagePlan_Investigate(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, investigatePlanJSON))
	gt.NoError(t, repo.Save(ctx, "ws/tk/plan", h))

	plan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, repo, "ws", "tk")).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanInvestigate)
	gt.NotNil(t, plan.Investigate)
	gt.N(t, len(plan.Investigate.Subtasks)).Equal(1)
	gt.S(t, plan.Investigate.Subtasks[0].Request).Equal("Collect related Slack posts")
	gt.S(t, plan.Message).Equal("Investigating now")
}

func TestLoadLatestTriagePlan_Ask(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustAssistantPlanMessage(t, askPlanJSON))
	gt.NoError(t, repo.Save(ctx, "ws/tk/plan", h))

	plan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, repo, "ws", "tk")).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanAsk)
	gt.N(t, len(plan.Ask.Questions)).Equal(1)
	gt.S(t, string(plan.Ask.Questions[0].ID)).Equal("q1")
}

func TestLoadLatestTriagePlan_PicksLatest(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages,
		mustAssistantPlanMessage(t, investigatePlanJSON),
		mustUserTextMessage(t, "Investigate result: ..."),
		mustAssistantPlanMessage(t, askPlanJSON),
	)
	gt.NoError(t, repo.Save(ctx, "ws/tk/plan", h))

	plan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, repo, "ws", "tk")).NoError(t)
	gt.Equal(t, plan.Kind, types.PlanAsk)
}

func TestIsWaitingUserSubmit(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()

	t.Run("empty history", func(t *testing.T) {
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-empty")).NoError(t)
		gt.False(t, ok)
	})

	t.Run("ask without user response", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages, mustAssistantPlanMessage(t, askPlanJSON))
		gt.NoError(t, repo.Save(ctx, "ws/tk-ask/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-ask")).NoError(t)
		gt.True(t, ok)
	})

	t.Run("ask followed by user response", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages,
			mustAssistantPlanMessage(t, askPlanJSON),
			mustUserTextMessage(t, "Q1=A1"),
		)
		gt.NoError(t, repo.Save(ctx, "ws/tk-answered/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-answered")).NoError(t)
		gt.False(t, ok)
	})

	t.Run("investigate trailing", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages, mustAssistantPlanMessage(t, investigatePlanJSON))
		gt.NoError(t, repo.Save(ctx, "ws/tk-inv/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-inv")).NoError(t)
		gt.False(t, ok)
	})
}

func TestCountPlannerTurns(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		n := gt.R1(triage.CountPlannerTurnsForTest(ctx, repo, "ws", "tk-empty")).NoError(t)
		gt.N(t, n).Equal(0)
	})

	t.Run("multiple plans", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages,
			mustAssistantPlanMessage(t, investigatePlanJSON),
			mustUserTextMessage(t, "Investigate result"),
			mustAssistantPlanMessage(t, askPlanJSON),
			mustUserTextMessage(t, "Q1=A1"),
			mustAssistantPlanMessage(t, completePlanJSON),
		)
		gt.NoError(t, repo.Save(ctx, "ws/tk-multi/plan", h))
		n := gt.R1(triage.CountPlannerTurnsForTest(ctx, repo, "ws", "tk-multi")).NoError(t)
		gt.N(t, n).Equal(3)
	})

	t.Run("ignores assistant text that is not a plan", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		c := gt.R1(gollem.NewTextContent("hello, just chatting")).NoError(t)
		h.Messages = append(h.Messages, gollem.Message{
			Role:     gollem.RoleAssistant,
			Contents: []gollem.MessageContent{c},
		})
		gt.NoError(t, repo.Save(ctx, "ws/tk-other/plan", h))
		n := gt.R1(triage.CountPlannerTurnsForTest(ctx, repo, "ws", "tk-other")).NoError(t)
		gt.N(t, n).Equal(0)
	})
}

func TestHasPlanHistory(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		ok := gt.R1(triage.HasPlanHistoryForTest(ctx, repo, "ws", "tk-empty")).NoError(t)
		gt.False(t, ok)
	})

	t.Run("after append", func(t *testing.T) {
		gt.NoError(t, triage.AppendUserMessageForTest(ctx, repo, "ws", "tk-here", "hello"))
		ok := gt.R1(triage.HasPlanHistoryForTest(ctx, repo, "ws", "tk-here")).NoError(t)
		gt.True(t, ok)
	})
}
