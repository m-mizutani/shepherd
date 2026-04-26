package triage

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
)

// llmPlan executes one planning turn against the LLM and returns the proposed
// TriagePlan.
//
// The implementation is deliberately a single Session.Generate call rather
// than a full agent.Execute loop: the planner needs exactly one propose_*
// tool call from the model, and Session.Generate already returns the model's
// FunctionCalls in its Response. Running an agent loop would (a) execute the
// propose_* tools (we don't want that — they're spec-only), and (b) require
// a sentinel error / side-channel state to short-circuit the loop. Decoding
// the FunctionCall directly from the response is straightforward and
// honest about what the planner is doing.
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

	sid := planSessionID(ticket.WorkspaceID, ticket.ID)
	history, err := e.historyRepo.Load(ctx, sid)
	if err != nil {
		return nil, goerr.Wrap(err, "load plan history",
			goerr.V("session_id", sid))
	}

	opts := []gollem.SessionOption{
		gollem.WithSessionSystemPrompt(systemPrompt),
		gollem.WithSessionTools(proposeTools()...),
	}
	if history != nil {
		opts = append(opts, gollem.WithSessionHistory(history))
	}
	session, err := e.llm.NewSession(ctx, opts...)
	if err != nil {
		return nil, goerr.Wrap(err, "new llm session")
	}

	// Non-empty kickoff text: Gemini's GenerateContent rejects empty parts.
	resp, err := session.Generate(ctx, []gollem.Input{
		gollem.Text("Decide and call exactly one of propose_investigate, propose_ask, or propose_complete based on the ticket and any prior context above."),
	})
	if err != nil {
		return nil, goerr.Wrap(err, "generate triage plan",
			goerr.V("ticket_id", ticket.ID))
	}
	if len(resp.FunctionCalls) == 0 {
		return nil, goerr.New("LLM did not call a propose_* tool",
			goerr.V("ticket_id", ticket.ID),
			goerr.V("texts", resp.Texts))
	}

	plan, err := decodePlanFromFunctionCall(resp.FunctionCalls[0])
	if err != nil {
		return nil, goerr.Wrap(err, "decode plan",
			goerr.V("ticket_id", ticket.ID))
	}

	// Persist the session's updated history (now including the LLM's
	// assistant tool-call message) so the next planning turn picks up
	// where this one left off.
	updated, err := session.History()
	if err != nil {
		return nil, goerr.Wrap(err, "read session history")
	}
	if err := e.historyRepo.Save(ctx, sid, updated); err != nil {
		return nil, goerr.Wrap(err, "save plan history",
			goerr.V("session_id", sid))
	}

	return plan, nil
}
