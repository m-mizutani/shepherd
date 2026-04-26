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

func mustToolCallMessage(t *testing.T, name string, args map[string]any) gollem.Message {
	t.Helper()
	c := gt.R1(gollem.NewToolCallContent("call-id", name, args)).NoError(t)
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

func investigateArgs() map[string]any {
	return map[string]any{
		"message": "調査します",
		"subtasks": []any{
			map[string]any{
				"id":                  "st1",
				"request":             "Collect related Slack posts",
				"acceptance_criteria": []any{"return at least 3 messages or explicit none"},
				"allowed_tools":       []any{"slack_search_messages"},
			},
		},
	}
}

func askArgs() map[string]any {
	return map[string]any{
		"message": "確認させてください",
		"title":   "詳細",
		"questions": []any{
			map[string]any{
				"id":    "q1",
				"label": "影響範囲は？",
				"choices": []any{
					map[string]any{"id": "c1", "label": "本番"},
					map[string]any{"id": "c2", "label": "ステージング"},
				},
			},
		},
	}
}

func completeArgs() map[string]any {
	return map[string]any{
		"message": "完了しました",
		"summary": "Investigation done",
		"assignee": map[string]any{
			"kind":      "assigned",
			"user_id":   "U123",
			"reasoning": "owner of the affected service",
		},
	}
}

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
	h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeInvestigateToolNameForTest, investigateArgs()))
	gt.NoError(t, repo.Save(ctx, "ws/tk/plan", h))

	plan := gt.R1(triage.LoadLatestTriagePlanForTest(ctx, repo, "ws", "tk")).NoError(t)
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanInvestigate)
	gt.NotNil(t, plan.Investigate)
	gt.N(t, len(plan.Investigate.Subtasks)).Equal(1)
	gt.S(t, plan.Investigate.Subtasks[0].Request).Equal("Collect related Slack posts")
	gt.S(t, plan.Message).Equal("調査します")
}

func TestLoadLatestTriagePlan_Ask(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()
	h := &gollem.History{Version: gollem.HistoryVersion}
	h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeAskToolNameForTest, askArgs()))
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
		mustToolCallMessage(t, triage.ProposeInvestigateToolNameForTest, investigateArgs()),
		mustUserTextMessage(t, "Investigate結果: ..."),
		mustToolCallMessage(t, triage.ProposeAskToolNameForTest, askArgs()),
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
		h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeAskToolNameForTest, askArgs()))
		gt.NoError(t, repo.Save(ctx, "ws/tk-ask/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-ask")).NoError(t)
		gt.True(t, ok)
	})

	t.Run("ask followed by user response", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages,
			mustToolCallMessage(t, triage.ProposeAskToolNameForTest, askArgs()),
			mustUserTextMessage(t, "Q1=A1"),
		)
		gt.NoError(t, repo.Save(ctx, "ws/tk-answered/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-answered")).NoError(t)
		gt.False(t, ok)
	})

	t.Run("investigate trailing", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages, mustToolCallMessage(t, triage.ProposeInvestigateToolNameForTest, investigateArgs()))
		gt.NoError(t, repo.Save(ctx, "ws/tk-inv/plan", h))
		ok := gt.R1(triage.IsWaitingUserSubmitForTest(ctx, repo, "ws", "tk-inv")).NoError(t)
		gt.False(t, ok)
	})
}

func TestCountToolCalls(t *testing.T) {
	repo := newFakeHistory()
	ctx := context.Background()

	t.Run("empty", func(t *testing.T) {
		n := gt.R1(triage.CountToolCallsForTest(ctx, repo, "ws", "tk-empty")).NoError(t)
		gt.N(t, n).Equal(0)
	})

	t.Run("multiple proposes", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages,
			mustToolCallMessage(t, triage.ProposeInvestigateToolNameForTest, investigateArgs()),
			mustUserTextMessage(t, "Investigate結果"),
			mustToolCallMessage(t, triage.ProposeAskToolNameForTest, askArgs()),
			mustUserTextMessage(t, "Q1=A1"),
			mustToolCallMessage(t, triage.ProposeCompleteToolNameForTest, completeArgs()),
		)
		gt.NoError(t, repo.Save(ctx, "ws/tk-multi/plan", h))
		n := gt.R1(triage.CountToolCallsForTest(ctx, repo, "ws", "tk-multi")).NoError(t)
		gt.N(t, n).Equal(3)
	})

	t.Run("ignores non-propose tools", func(t *testing.T) {
		h := &gollem.History{Version: gollem.HistoryVersion}
		h.Messages = append(h.Messages, mustToolCallMessage(t, "slack_search_messages", map[string]any{"query": "x"}))
		gt.NoError(t, repo.Save(ctx, "ws/tk-other/plan", h))
		n := gt.R1(triage.CountToolCallsForTest(ctx, repo, "ws", "tk-other")).NoError(t)
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
