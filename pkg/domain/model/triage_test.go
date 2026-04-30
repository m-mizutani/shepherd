package model_test

import (
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

func validInvestigatePlan() *model.TriagePlan {
	uid := types.SlackUserID("U1")
	_ = uid
	return &model.TriagePlan{
		Kind:    types.PlanInvestigate,
		Message: "調査を開始します",
		Investigate: &model.Investigate{
			Subtasks: []model.Subtask{
				{
					ID:                 "st1",
					Request:            "Collect related Slack posts",
					AcceptanceCriteria: []string{"Return up to 10 messages"},
					AllowedTools:       []string{"slack_search_messages"},
				},
			},
		},
	}
}

func validAskPlan() *model.TriagePlan {
	return &model.TriagePlan{
		Kind:    types.PlanAsk,
		Message: "確認させてください",
		Ask: &model.Ask{
			Title: "詳細確認",
			Questions: []model.Question{
				{
					ID:    "q1",
					Label: "影響範囲は？",
					Choices: []model.Choice{
						{ID: "c1", Label: "本番"},
						{ID: "c2", Label: "ステージング"},
					},
				},
			},
		},
	}
}

func validCompletePlan() *model.TriagePlan {
	return &model.TriagePlan{
		Kind:    types.PlanComplete,
		Message: "triage completed",
		Complete: &model.Complete{
			Description: "Investigation done",
			Assignee: model.AssigneeDecision{
				Kind:      types.AssigneeAssigned,
				UserIDs:   []types.SlackUserID{"U1"},
				Reasoning: "responsible for the affected service",
			},
		},
	}
}

func TestTriagePlanValidate_Valid(t *testing.T) {
	t.Run("investigate", func(t *testing.T) {
		gt.NoError(t, validInvestigatePlan().Validate())
	})
	t.Run("ask", func(t *testing.T) {
		gt.NoError(t, validAskPlan().Validate())
	})
	t.Run("complete", func(t *testing.T) {
		gt.NoError(t, validCompletePlan().Validate())
	})
}

func TestTriagePlanValidate_NilOrEmpty(t *testing.T) {
	t.Run("nil plan", func(t *testing.T) {
		var p *model.TriagePlan
		gt.Error(t, p.Validate())
	})
	t.Run("empty message", func(t *testing.T) {
		p := validInvestigatePlan()
		p.Message = ""
		err := p.Validate()
		gt.Error(t, err)
		gt.S(t, err.Error()).Contains("message")
	})
	t.Run("unknown kind", func(t *testing.T) {
		p := validInvestigatePlan()
		p.Kind = types.PlanKind("bogus")
		gt.Error(t, p.Validate())
	})
}

func TestTriagePlanValidate_PayloadMismatch(t *testing.T) {
	t.Run("investigate kind missing payload", func(t *testing.T) {
		p := validInvestigatePlan()
		p.Investigate = nil
		gt.Error(t, p.Validate())
	})
	t.Run("ask kind with extra investigate payload", func(t *testing.T) {
		p := validAskPlan()
		p.Investigate = validInvestigatePlan().Investigate
		gt.Error(t, p.Validate())
	})
	t.Run("complete kind missing payload", func(t *testing.T) {
		p := validCompletePlan()
		p.Complete = nil
		gt.Error(t, p.Validate())
	})
}

func TestInvestigateValidate(t *testing.T) {
	t.Run("empty subtasks", func(t *testing.T) {
		gt.Error(t, (&model.Investigate{}).Validate())
	})
	t.Run("invalid subtask propagates", func(t *testing.T) {
		inv := &model.Investigate{Subtasks: []model.Subtask{{ID: "", Request: "x", AcceptanceCriteria: []string{"y"}}}}
		gt.Error(t, inv.Validate())
	})
	t.Run("duplicate subtask id", func(t *testing.T) {
		st := model.Subtask{ID: "a", Request: "r", AcceptanceCriteria: []string{"c"}}
		inv := &model.Investigate{Subtasks: []model.Subtask{st, st}}
		err := inv.Validate()
		gt.Error(t, err)
		gt.S(t, err.Error()).Contains("duplicate")
	})
}

func TestSubtaskValidate(t *testing.T) {
	base := model.Subtask{
		ID:                 "id",
		Request:            "Do X",
		AcceptanceCriteria: []string{"return Y"},
	}
	gt.NoError(t, base.Validate())

	t.Run("missing request", func(t *testing.T) {
		s := base
		s.Request = ""
		gt.Error(t, s.Validate())
	})
	t.Run("missing criteria", func(t *testing.T) {
		s := base
		s.AcceptanceCriteria = nil
		gt.Error(t, s.Validate())
	})
	t.Run("empty id", func(t *testing.T) {
		s := base
		s.ID = ""
		gt.Error(t, s.Validate())
	})
}

func TestAskValidate(t *testing.T) {
	t.Run("empty questions", func(t *testing.T) {
		gt.Error(t, (&model.Ask{}).Validate())
	})
	t.Run("duplicate question id", func(t *testing.T) {
		q := model.Question{ID: "q", Label: "L", Choices: []model.Choice{{ID: "c", Label: "C"}}}
		ask := &model.Ask{Questions: []model.Question{q, q}}
		err := ask.Validate()
		gt.Error(t, err)
		gt.S(t, err.Error()).Contains("duplicate question")
	})
	t.Run("invalid question propagates", func(t *testing.T) {
		ask := &model.Ask{Questions: []model.Question{{ID: "q", Label: ""}}}
		gt.Error(t, ask.Validate())
	})
}

func TestQuestionValidate(t *testing.T) {
	base := model.Question{
		ID:      "q",
		Label:   "L",
		Choices: []model.Choice{{ID: "c1", Label: "C1"}},
	}
	gt.NoError(t, base.Validate())

	t.Run("empty choices", func(t *testing.T) {
		q := base
		q.Choices = nil
		gt.Error(t, q.Validate())
	})
	t.Run("duplicate choice id", func(t *testing.T) {
		q := base
		q.Choices = []model.Choice{{ID: "c", Label: "A"}, {ID: "c", Label: "B"}}
		gt.Error(t, q.Validate())
	})
}

func TestAnswerIsValid(t *testing.T) {
	t.Run("selected only", func(t *testing.T) {
		gt.True(t, (&model.Answer{SelectedIDs: []types.ChoiceID{"c1"}}).IsValid())
	})
	t.Run("text only", func(t *testing.T) {
		gt.True(t, (&model.Answer{OtherText: "hello"}).IsValid())
	})
	t.Run("text whitespace only", func(t *testing.T) {
		gt.False(t, (&model.Answer{OtherText: "  \t \n"}).IsValid())
	})
	t.Run("empty", func(t *testing.T) {
		gt.False(t, (&model.Answer{}).IsValid())
	})
}

func TestCompleteValidate(t *testing.T) {
	base := func() *model.Complete {
		return &model.Complete{
			Description: "description",
			Assignee: model.AssigneeDecision{
				Kind:      types.AssigneeAssigned,
				UserIDs:   []types.SlackUserID{"U1"},
				Reasoning: "reason",
			},
		}
	}
	gt.NoError(t, base().Validate())

	t.Run("empty description", func(t *testing.T) {
		c := base()
		c.Description = ""
		gt.Error(t, c.Validate())
	})
	t.Run("invalid assignee propagates", func(t *testing.T) {
		c := base()
		c.Assignee.Reasoning = ""
		gt.Error(t, c.Validate())
	})
}

func TestAssigneeDecisionValidate(t *testing.T) {
	t.Run("assigned single ok", func(t *testing.T) {
		gt.NoError(t, (&model.AssigneeDecision{
			Kind: types.AssigneeAssigned, UserIDs: []types.SlackUserID{"U1"}, Reasoning: "r",
		}).Validate())
	})
	t.Run("assigned multiple ok", func(t *testing.T) {
		gt.NoError(t, (&model.AssigneeDecision{
			Kind:      types.AssigneeAssigned,
			UserIDs:   []types.SlackUserID{"U1", "U2", "U3"},
			Reasoning: "r",
		}).Validate())
	})
	t.Run("assigned with empty list rejected", func(t *testing.T) {
		gt.Error(t, (&model.AssigneeDecision{
			Kind: types.AssigneeAssigned, UserIDs: nil, Reasoning: "r",
		}).Validate())
	})
	t.Run("assigned with empty id rejected", func(t *testing.T) {
		gt.Error(t, (&model.AssigneeDecision{
			Kind: types.AssigneeAssigned, UserIDs: []types.SlackUserID{"U1", ""}, Reasoning: "r",
		}).Validate())
	})
	t.Run("unassigned ok", func(t *testing.T) {
		gt.NoError(t, (&model.AssigneeDecision{
			Kind: types.AssigneeUnassigned, Reasoning: "needs team review",
		}).Validate())
	})
	t.Run("unassigned with users rejected", func(t *testing.T) {
		gt.Error(t, (&model.AssigneeDecision{
			Kind: types.AssigneeUnassigned, UserIDs: []types.SlackUserID{"U1"}, Reasoning: "r",
		}).Validate())
	})
	t.Run("missing reasoning", func(t *testing.T) {
		gt.Error(t, (&model.AssigneeDecision{
			Kind: types.AssigneeAssigned, UserIDs: []types.SlackUserID{"U1"},
		}).Validate())
	})
	t.Run("unknown kind", func(t *testing.T) {
		gt.Error(t, (&model.AssigneeDecision{
			Kind: types.AssigneeDecisionKind("bogus"), Reasoning: "r",
		}).Validate())
	})

	// satisfies importing strings package (avoid unused warnings if referenced via gt.S above).
	_ = strings.HasPrefix
}
