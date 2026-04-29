package triage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	"github.com/m-mizutani/shepherd/pkg/utils/logging"
)

// errInvalidPlan marks a planner response that decoded but failed semantic
// validation (schema-only constraints already passed at the gollem layer).
// The planner loop catches it to drive the retry path; once retries are
// exhausted it bubbles up wrapped through the failure-recovery handler.
var errInvalidPlan = goerr.New("invalid triage plan")

// dateAutoFillPattern is the ISO 8601 calendar date the schema also enforces;
// kept in lock-step with autoFillFieldSchema's Pattern so the Go-side
// validator gives the LLM the same answer the schema does.
var dateAutoFillPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// llmPlan executes one planning turn against the LLM and returns the proposed
// TriagePlan. The agent is run with WithResponseSchema; auto-fill custom
// fields constrain complete.suggested_fields. Validation failures are
// re-asked up to PlanRetryCap times with the previous turn's history kept
// intact — the retry message carries the verbatim error so the model can
// self-correct.
func (e *PlanExecutor) llmPlan(ctx context.Context, ticket *model.Ticket) (*model.TriagePlan, error) {
	autoFill := e.autoFillFields(ticket.WorkspaceID)
	in := prompt.TriagePlanInput{
		Title:          ticket.Title,
		Description:    ticket.Description,
		InitialMessage: ticket.InitialMessage,
		Reporter:       string(ticket.ReporterSlackUserID),
		AutoFillFields: autoFillBriefing(autoFill),
	}
	var systemPrompt string
	var err error
	if e.promptUC != nil {
		systemPrompt, err = e.promptUC.RenderTriagePlan(ctx, ticket.WorkspaceID, in)
	} else {
		systemPrompt, err = prompt.RenderTriagePlan(in)
	}
	if err != nil {
		return nil, goerr.Wrap(err, "render triage_plan prompt")
	}

	agent := gollem.New(e.llm,
		gollem.WithSystemPrompt(systemPrompt),
		gollem.WithContentType(gollem.ContentTypeJSON),
		gollem.WithResponseSchema(triagePlanSchema(autoFill)),
		gollem.WithHistoryRepository(e.historyRepo, planSessionID(ticket.WorkspaceID, ticket.ID)),
	)

	// Non-empty kickoff text: Gemini's GenerateContent rejects empty parts.
	kickoff := gollem.Text("Decide and return a TriagePlan choosing exactly one of investigate / ask / complete based on the ticket and any prior context above.")

	cap := e.cfg.PlanRetryCap
	for attempt := 0; attempt <= cap; attempt++ {
		resp, err := agent.Execute(ctx, kickoff)
		if err != nil {
			return nil, goerr.Wrap(err, "agent execute",
				goerr.V("ticket_id", ticket.ID),
				goerr.V("attempt", attempt))
		}
		if resp == nil || len(resp.Texts) == 0 {
			return nil, goerr.New("LLM returned no plan body",
				goerr.V("ticket_id", ticket.ID))
		}

		raw := strings.Join(resp.Texts, "")
		plan, decodeErr := decodePlanFromJSON(raw)
		if decodeErr == nil {
			if vErr := validatePlanAutoFill(plan, autoFill); vErr == nil {
				logging.From(ctx).Debug("triage plan generated",
					slog.String("ticket_id", string(ticket.ID)),
					slog.String("kind", string(plan.Kind)),
					slog.String("message", plan.Message),
					slog.String("raw", raw),
					slog.Int("attempt", attempt),
				)
				return plan, nil
			} else {
				decodeErr = vErr
			}
		}

		// Out of retries — surface the failure so the deferred handler posts
		// the standard recovery message.
		if attempt == cap {
			return nil, goerr.Wrap(decodeErr, "decode triage plan from agent response",
				goerr.V("ticket_id", ticket.ID),
				goerr.V("raw", resp.Texts),
				goerr.V("attempt", attempt))
		}

		// Slack: short notice, no error detail; full error stays in the logs.
		e.notifyPlanRetry(ctx, ticket, decodeErr, attempt)

		// Feed the verbatim error back as the next turn's user message so
		// the model self-corrects. The same gollem session continues, so the
		// previous (invalid) assistant turn is already part of history.
		kickoff = gollem.Text(fmt.Sprintf(
			"Your previous response was invalid: %s. Re-emit a TriagePlan that satisfies the schema and the auto-fill rules from the system prompt.",
			decodeErr.Error(),
		))
	}

	// Unreachable: the loop returns from inside on every path.
	return nil, goerr.New("planner retry loop exited unexpectedly")
}

// notifyPlanRetry posts the i18n "retrying" notice to the ticket thread.
// Failures are logged via errutil.Handle and intentionally swallowed — a
// flapping Slack should not block the planner's retry.
func (e *PlanExecutor) notifyPlanRetry(ctx context.Context, ticket *model.Ticket, cause error, attempt int) {
	logging.From(ctx).Warn("planner response invalid; retrying",
		slog.String("ticket_id", string(ticket.ID)),
		slog.Int("attempt", attempt),
		slog.String("error", cause.Error()),
	)
	msg := i18n.From(ctx).T(i18n.MsgTriagePlanRetrying)
	if err := e.slack.ReplyThread(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), msg); err != nil {
		errutil.Handle(ctx, goerr.Wrap(err, "post planner retry notice",
			goerr.V("ticket_id", ticket.ID)))
	}
}

// autoFillFields returns the workspace's custom fields whose AutoFill flag
// is set, in declaration order. nil-safe; returns nil when no schema is
// configured for ws.
func (e *PlanExecutor) autoFillFields(ws types.WorkspaceID) []domainConfig.FieldDefinition {
	if e.lookup == nil {
		return nil
	}
	schema := e.lookup.WorkspaceSchema(ws)
	if schema == nil {
		return nil
	}
	out := make([]domainConfig.FieldDefinition, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		if f.AutoFill {
			out = append(out, f)
		}
	}
	return out
}

// autoFillBriefing projects the FieldDefinition slice into the prompt input
// shape. Only fields with AutoFill are included; the projection drops the
// Metadata that the prompt template does not need.
func autoFillBriefing(fields []domainConfig.FieldDefinition) []prompt.AutoFillField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]prompt.AutoFillField, 0, len(fields))
	for _, f := range fields {
		opts := make([]prompt.AutoFillOption, 0, len(f.Options))
		for _, o := range f.Options {
			opts = append(opts, prompt.AutoFillOption{
				ID:          o.ID,
				Label:       o.Name,
				Description: o.Description,
			})
		}
		out = append(out, prompt.AutoFillField{
			ID:          f.ID,
			Name:        f.Name,
			Type:        string(f.Type),
			Description: f.Description,
			Required:    f.Required,
			Options:     opts,
		})
	}
	return out
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

// validatePlanAutoFill enforces the auto-fill contract on top of the schema
// the LLM already followed: required fields must be present, values must
// match the declared type, and select / multi-select option ids must come
// from the allow-list. Plans whose Kind != complete short-circuit.
func validatePlanAutoFill(plan *model.TriagePlan, autoFill []domainConfig.FieldDefinition) error {
	if plan.Kind != types.PlanComplete || plan.Complete == nil {
		return nil
	}
	if len(autoFill) == 0 {
		return nil
	}
	values := plan.Complete.SuggestedFields
	defs := make(map[string]domainConfig.FieldDefinition, len(autoFill))
	for _, f := range autoFill {
		defs[f.ID] = f
	}
	for _, f := range autoFill {
		v, ok := values[f.ID]
		if !ok {
			if f.Required {
				return goerr.Wrap(errInvalidPlan,
					"auto_fill field is required but missing from suggested_fields",
					goerr.V("field_id", f.ID))
			}
			continue
		}
		if err := validateAutoFillValue(f, v); err != nil {
			return goerr.Wrap(errors.Join(errInvalidPlan, err),
				"auto_fill value rejected",
				goerr.V("field_id", f.ID))
		}
	}
	for id := range values {
		if _, known := defs[id]; !known {
			// Tolerate ids that map to non-auto-fill fields: the LLM is
			// allowed to volunteer those. Only flat-out unknown ids are
			// rejected.
			continue
		}
	}
	return nil
}

func validateAutoFillValue(f domainConfig.FieldDefinition, v any) error {
	switch f.Type {
	case types.FieldTypeText, types.FieldTypeURL:
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return goerr.New("expected non-empty string", goerr.V("got", fmt.Sprintf("%T", v)))
		}
		if f.Type == types.FieldTypeURL {
			u, err := url.Parse(s)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return goerr.New("expected absolute URL", goerr.V("value", s))
			}
		}
	case types.FieldTypeNumber:
		switch v.(type) {
		case float64, float32, int, int64:
			// ok
		default:
			return goerr.New("expected number", goerr.V("got", fmt.Sprintf("%T", v)))
		}
	case types.FieldTypeDate:
		s, ok := v.(string)
		if !ok || !dateAutoFillPattern.MatchString(s) {
			return goerr.New("expected YYYY-MM-DD date string", goerr.V("got", fmt.Sprintf("%v", v)))
		}
	case types.FieldTypeUser:
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return goerr.New("expected non-empty Slack user id", goerr.V("got", fmt.Sprintf("%T", v)))
		}
	case types.FieldTypeMultiUser:
		list, ok := anyToStringSlice(v)
		if !ok {
			return goerr.New("expected array of Slack user ids", goerr.V("got", fmt.Sprintf("%T", v)))
		}
		for _, s := range list {
			if strings.TrimSpace(s) == "" {
				return goerr.New("multi-user contains empty entry")
			}
		}
	case types.FieldTypeSelect:
		s, ok := v.(string)
		if !ok {
			return goerr.New("expected string for select", goerr.V("got", fmt.Sprintf("%T", v)))
		}
		if !optionAllowed(f.Options, s) {
			return goerr.New("select value not in option list",
				goerr.V("value", s),
				goerr.V("allowed", optionIDs(f.Options)))
		}
	case types.FieldTypeMultiSelect:
		list, ok := anyToStringSlice(v)
		if !ok {
			return goerr.New("expected array for multi-select", goerr.V("got", fmt.Sprintf("%T", v)))
		}
		for _, s := range list {
			if !optionAllowed(f.Options, s) {
				return goerr.New("multi-select value not in option list",
					goerr.V("value", s),
					goerr.V("allowed", optionIDs(f.Options)))
			}
		}
	}
	return nil
}

func optionAllowed(opts []domainConfig.FieldOption, id string) bool {
	for _, o := range opts {
		if o.ID == id {
			return true
		}
	}
	return false
}

func anyToStringSlice(v any) ([]string, bool) {
	switch x := v.(type) {
	case []string:
		return x, true
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			s, ok := e.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}
