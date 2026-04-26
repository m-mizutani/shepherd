package triage

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// llmPlan executes one planning turn against the LLM and returns the proposed
// TriagePlan.
//
// We let the agent shape its answer through WithResponseSchema (JSON output
// constrained to triagePlanSchema) and read the rendered JSON straight off
// agent.Execute's *ExecuteResponse. No tools, no agent-loop side-channels,
// no sentinel errors — the planner's output IS the agent's return value.
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

	agent := gollem.New(e.llm,
		gollem.WithSystemPrompt(systemPrompt),
		gollem.WithContentType(gollem.ContentTypeJSON),
		gollem.WithResponseSchema(triagePlanSchema()),
		gollem.WithHistoryRepository(e.historyRepo, planSessionID(ticket.WorkspaceID, ticket.ID)),
	)

	// Non-empty kickoff text: Gemini's GenerateContent rejects empty parts.
	resp, err := agent.Execute(ctx, gollem.Text("Decide and return a TriagePlan choosing exactly one of investigate / ask / complete based on the ticket and any prior context above."))
	if err != nil {
		return nil, goerr.Wrap(err, "agent execute",
			goerr.V("ticket_id", ticket.ID))
	}
	if resp == nil || len(resp.Texts) == 0 {
		return nil, goerr.New("LLM returned no plan body",
			goerr.V("ticket_id", ticket.ID))
	}

	raw := strings.Join(resp.Texts, "")
	plan, err := decodePlanFromJSON(raw)
	if err != nil {
		return nil, goerr.Wrap(err, "decode triage plan from agent response",
			goerr.V("ticket_id", ticket.ID),
			goerr.V("raw", resp.Texts))
	}
	logging.From(ctx).Debug("triage plan generated",
		slog.String("ticket_id", string(ticket.ID)),
		slog.String("kind", string(plan.Kind)),
		slog.String("message", plan.Message),
		slog.String("raw", raw),
	)
	return plan, nil
}

// decodePlanFromJSON parses the structured JSON the LLM produced under
// triagePlanSchema. Validation rejects the half-populated unions the schema
// alone cannot enforce (e.g. kind=ask without an ask payload).
func decodePlanFromJSON(raw string) (*model.TriagePlan, error) {
	plan := &model.TriagePlan{}
	if err := json.NewDecoder(strings.NewReader(raw)).Decode(plan); err != nil {
		return nil, goerr.Wrap(err, "json decode")
	}
	if err := plan.Validate(); err != nil {
		return nil, goerr.Wrap(err, "invalid plan")
	}
	return plan, nil
}
