package triage_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
)

func TestProposeTools_SpecValid(t *testing.T) {
	tools := triage.ProposeToolsForTest()
	gt.N(t, len(tools)).Equal(3)

	names := []string{
		triage.ProposeInvestigateToolNameForTest,
		triage.ProposeAskToolNameForTest,
		triage.ProposeCompleteToolNameForTest,
	}
	for i, tl := range tools {
		spec := tl.Spec()
		gt.NoError(t, spec.Validate())
		gt.S(t, spec.Name).Equal(names[i])
	}
}

func TestProposeTools_RunIsNotInvokable(t *testing.T) {
	// The propose_* tools are spec-only: llmPlan reads the LLM's
	// FunctionCall directly from Session.Generate's response and never
	// drives an agent loop over them. Calling Run is therefore a bug,
	// and the tools surface that explicitly.
	for _, tl := range triage.ProposeToolsForTest() {
		_, err := tl.Run(context.Background(), map[string]any{})
		gt.Error(t, err)
	}
}

func TestDecodePlanFromFunctionCall_Investigate(t *testing.T) {
	plan, err := triage.DecodePlanFromFunctionCallForTest(&gollem.FunctionCall{
		ID: "fc1", Name: triage.ProposeInvestigateToolNameForTest, Arguments: investigateArgs(),
	})
	gt.NoError(t, err)
	gt.Equal(t, plan.Kind, types.PlanInvestigate)
	gt.S(t, plan.Message).Equal("調査します")
	gt.NotNil(t, plan.Investigate)
	gt.N(t, len(plan.Investigate.Subtasks)).Equal(1)
	gt.S(t, plan.Investigate.Subtasks[0].Request).Equal("Collect related Slack posts")
}

func TestDecodePlanFromFunctionCall_Ask(t *testing.T) {
	plan, err := triage.DecodePlanFromFunctionCallForTest(&gollem.FunctionCall{
		ID: "fc2", Name: triage.ProposeAskToolNameForTest, Arguments: askArgs(),
	})
	gt.NoError(t, err)
	gt.Equal(t, plan.Kind, types.PlanAsk)
	gt.NotNil(t, plan.Ask)
	gt.N(t, len(plan.Ask.Questions)).Equal(1)
}

func TestDecodePlanFromFunctionCall_Complete(t *testing.T) {
	plan, err := triage.DecodePlanFromFunctionCallForTest(&gollem.FunctionCall{
		ID: "fc3", Name: triage.ProposeCompleteToolNameForTest, Arguments: completeArgs(),
	})
	gt.NoError(t, err)
	gt.Equal(t, plan.Kind, types.PlanComplete)
	gt.NotNil(t, plan.Complete)
	gt.S(t, plan.Complete.Summary).Equal("Investigation done")
	gt.Equal(t, plan.Complete.Assignee.Kind, types.AssigneeAssigned)
	gt.NotNil(t, plan.Complete.Assignee.UserID)
	gt.S(t, string(*plan.Complete.Assignee.UserID)).Equal("U123")
}

func TestDecodePlanFromFunctionCall_RejectsInvalidPlan(t *testing.T) {
	args := investigateArgs()
	delete(args, "message")
	_, err := triage.DecodePlanFromFunctionCallForTest(&gollem.FunctionCall{
		ID: "fc-bad", Name: triage.ProposeInvestigateToolNameForTest, Arguments: args,
	})
	gt.Error(t, err)
}

func TestDecodePlanFromFunctionCall_UnknownToolName(t *testing.T) {
	_, err := triage.DecodePlanFromFunctionCallForTest(&gollem.FunctionCall{
		ID: "fc-unk", Name: "propose_unknown", Arguments: map[string]any{"message": "x"},
	})
	gt.Error(t, err)
}
