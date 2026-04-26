package triage

import (
	"context"
	"errors"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
)

// llmPlan executes one planning turn against the LLM and returns the
// proposed TriagePlan. The prior turn's user-facing context (investigation
// summaries, reporter answers) must be appended to history via
// AppendUserMessage before this is called; the gollem agent loads that
// history from sessionID and asks the LLM what to do next.
//
// Internally it wires three propose_* tools to a fresh PlanCapture, runs
// agent.Execute(""), and reads back the captured plan. The propose_* tools
// return errPlanProposed to short-circuit the agent loop after the first
// (and only) tool call.
func (e *PlanExecutor) llmPlan(ctx context.Context, ticket *model.Ticket) (*model.TriagePlan, error) {
	systemPrompt, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title:          ticket.Title,
		Description:    ticket.Description,
		InitialMessage: ticket.InitialMessage,
		Reporter:       string(ticket.ReporterSlackUserID),
	})
	if err != nil {
		return nil, goerr.Wrap(err, "render triage_plan prompt")
	}

	capture := &PlanCapture{}
	tools := ProposeTools(capture)

	agent := gollem.New(e.llm,
		gollem.WithSystemPrompt(systemPrompt),
		gollem.WithTools(tools...),
		gollem.WithHistoryRepository(e.historyRepo, PlanSessionID(ticket.WorkspaceID, ticket.ID)),
	)

	// Execute returns errPlanProposed after the LLM picks an action. That is
	// the success path; surface every other error.
	if _, err := agent.Execute(ctx, gollem.Text("")); err != nil {
		if !errors.Is(err, errPlanProposed) {
			return nil, goerr.Wrap(err, "agent execute",
				goerr.V("ticket_id", ticket.ID))
		}
	}

	plan := capture.Plan()
	if plan == nil {
		return nil, goerr.New("LLM did not propose any plan", goerr.V("ticket_id", ticket.ID))
	}
	return plan, nil
}

