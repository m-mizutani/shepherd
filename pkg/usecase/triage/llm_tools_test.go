package triage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/triage"
)

func TestProposeInvestigate_Capture(t *testing.T) {
	cap := &triage.PlanCaptureForTest{}
	tools := triage.ProposeToolsForTest(cap)
	gt.N(t, len(tools)).Equal(3)

	var inv = tools[0]
	gt.S(t, inv.Spec().Name).Equal(triage.ProposeInvestigateToolNameForTest)

	out, err := inv.Run(context.Background(), investigateArgs())
	// Run is expected to return a sentinel error to terminate the agent loop.
	gt.True(t, err != nil)
	gt.Equal(t, out["accepted"], true)

	plan := cap.PlanForTest()
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanInvestigate)
	gt.S(t, plan.Message).Equal("調査します")
	gt.NotNil(t, plan.Investigate)
	gt.N(t, len(plan.Investigate.Subtasks)).Equal(1)
	gt.S(t, plan.Investigate.Subtasks[0].Request).Equal("Collect related Slack posts")
}

func TestProposeAsk_Capture(t *testing.T) {
	cap := &triage.PlanCaptureForTest{}
	tools := triage.ProposeToolsForTest(cap)
	ask := tools[1]
	gt.S(t, ask.Spec().Name).Equal(triage.ProposeAskToolNameForTest)

	_, err := ask.Run(context.Background(), askArgs())
	gt.True(t, err != nil)

	plan := cap.PlanForTest()
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanAsk)
	gt.NotNil(t, plan.Ask)
	gt.N(t, len(plan.Ask.Questions)).Equal(1)
}

func TestProposeComplete_Capture(t *testing.T) {
	cap := &triage.PlanCaptureForTest{}
	tools := triage.ProposeToolsForTest(cap)
	comp := tools[2]
	gt.S(t, comp.Spec().Name).Equal(triage.ProposeCompleteToolNameForTest)

	_, err := comp.Run(context.Background(), completeArgs())
	gt.True(t, err != nil)

	plan := cap.PlanForTest()
	gt.NotNil(t, plan)
	gt.Equal(t, plan.Kind, types.PlanComplete)
	gt.NotNil(t, plan.Complete)
	gt.S(t, plan.Complete.Summary).Equal("Investigation done")
	gt.Equal(t, plan.Complete.Assignee.Kind, types.AssigneeAssigned)
	gt.NotNil(t, plan.Complete.Assignee.UserID)
	gt.S(t, string(*plan.Complete.Assignee.UserID)).Equal("U123")
}

func TestPropose_RejectInvalidPlan(t *testing.T) {
	// missing message - validation should fail.
	cap := &triage.PlanCaptureForTest{}
	tools := triage.ProposeToolsForTest(cap)
	args := investigateArgs()
	delete(args, "message")
	_, err := tools[0].Run(context.Background(), args)
	gt.Error(t, err)
	// Sentinel "plan proposed" error must NOT be the cause; we expect a real validation failure.
	gt.False(t, errors.Is(err, errors.New("triage plan proposed")))
	gt.Nil(t, cap.PlanForTest())
}

func TestPropose_OnlyOneAcceptedPerCapture(t *testing.T) {
	cap := &triage.PlanCaptureForTest{}
	tools := triage.ProposeToolsForTest(cap)
	_, err := tools[0].Run(context.Background(), investigateArgs())
	gt.True(t, err != nil)

	// second call: must error out, plan must remain the first one.
	_, err = tools[1].Run(context.Background(), askArgs())
	gt.Error(t, err)
	gt.Equal(t, cap.PlanForTest().Kind, types.PlanInvestigate)
}

func TestProposeSpecValidate(t *testing.T) {
	cap := &triage.PlanCaptureForTest{}
	for _, tool := range triage.ProposeToolsForTest(cap) {
		spec := tool.Spec()
		gt.NoError(t, spec.Validate())
	}
}
