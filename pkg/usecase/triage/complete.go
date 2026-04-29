package triage

import (
	"context"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	domainConfig "github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
	slackgo "github.com/slack-go/slack"
)

// finalizeComplete persists the LLM's (or human-edited) Complete onto the
// ticket and flips Triaged → true. It does NOT post any Slack message — the
// caller is responsible for the appropriate follow-up so we don't double-post
// (legacy fast path uses BuildCompleteBlocks; the review-submit path uses
// BuildReviewSubmittedBlocks).
//
// The Complete is passed in directly rather than reconstructed from the plan,
// so the same code path serves both the legacy "PlanComplete from planner"
// route and the reporter-review "Submit / Edit submit" route, where the
// Complete may have been edited by a human.
func (e *PlanExecutor) finalizeComplete(ctx context.Context, ticket *model.Ticket, comp *model.Complete) error {
	if comp == nil {
		return goerr.New("finalizeComplete called with nil Complete")
	}

	var assignee *types.SlackUserID
	if comp.Assignee.Kind == types.AssigneeAssigned && comp.Assignee.UserID != nil {
		assignee = comp.Assignee.UserID
	}

	// Persist the LLM's (or human-edited) Title and Summary as the ticket's
	// Title / Description so the values visible in the review message become
	// the authoritative ticket body. We do this BEFORE FinalizeTriage so a
	// failure leaves the ticket cleanly un-finalised. Empty Title leaves the
	// existing ticket.Title intact (older plans pre-date the field).
	titleChanged := strings.TrimSpace(comp.Title) != "" && ticket.Title != comp.Title
	descChanged := strings.TrimSpace(comp.Summary) != "" && ticket.Description != comp.Summary

	// Promote the planner's auto-filled suggestions into the ticket's
	// FieldValues so the web UI / hand-off see real values, not "—". Skipped
	// when the operator already set values via the Edit modal — those are
	// pre-merged into ticket.FieldValues by the caller, and FieldValues
	// already contains the operator's choice for that field.
	var schema *domainConfig.FieldSchema
	if e.lookup != nil {
		schema = e.lookup.WorkspaceSchema(ticket.WorkspaceID)
	}
	fieldsChanged := mergeSuggestedFields(ticket, comp, schema)

	if titleChanged || descChanged || fieldsChanged {
		if titleChanged {
			ticket.Title = comp.Title
		}
		if descChanged {
			ticket.Description = comp.Summary
		}
		if _, err := e.repo.Ticket().Update(ctx, ticket.WorkspaceID, ticket); err != nil {
			return goerr.Wrap(err, "persist triage title/description", goerr.V("ticket_id", ticket.ID))
		}
	}

	history := &model.TicketHistory{
		Action:    "triage_completed",
		ChangedBy: ticket.ReporterSlackUserID, // best-effort: the bot has no Slack user id of its own.
	}

	if err := e.repo.Ticket().FinalizeTriage(ctx, ticket.WorkspaceID, ticket.ID, assignee, history); err != nil {
		return goerr.Wrap(err, "finalize triage in repository", goerr.V("ticket_id", ticket.ID))
	}
	return nil
}

// finalizeCompleteAndAnnounce is the legacy fast-path entry: persist + post
// the BuildCompleteBlocks hand-off message. Used when the workspace has
// auto = true (planner converges → finalize immediately, no human review).
func (e *PlanExecutor) finalizeCompleteAndAnnounce(ctx context.Context, ticket *model.Ticket, comp *model.Complete) error {
	if err := e.finalizeComplete(ctx, ticket, comp); err != nil {
		return err
	}
	var schema *domainConfig.FieldSchema
	if e.lookup != nil {
		schema = e.lookup.WorkspaceSchema(ticket.WorkspaceID)
	}
	blocks := slackService.BuildCompleteBlocks(ctx, e.ticketRef(ticket), comp, schema, ticket.FieldValues)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage complete message")
	}
	return nil
}

// enterReview is the gated counterpart to finalizeComplete. The planner has
// proposed a Complete but the workspace requires a human to confirm before
// the ticket is finalised. We post the proposal as a review message carrying
// Edit / Submit / Re-investigate buttons; the ticket stays Triaged=false so
// loadLatestTriagePlan + Triaged-flag checks remain the single source of
// truth — no PendingTriage snapshot is persisted.
func (e *PlanExecutor) enterReview(ctx context.Context, ticket *model.Ticket, comp *model.Complete) error {
	if comp == nil {
		return goerr.New("enterReview called with nil Complete")
	}
	var schema *domainConfig.FieldSchema
	if e.lookup != nil {
		schema = e.lookup.WorkspaceSchema(ticket.WorkspaceID)
	}
	// Promote auto-filled suggestions to the ticket so the web UI and any
	// other observers see real values during the review window. Triaged
	// stays false (the buttons are still load-bearing); Submit / Edit-Submit
	// finalise. Operator-edited values from a prior Edit click are
	// preserved by the merge's "skip if already set" rule.
	if mergeSuggestedFields(ticket, comp, schema) {
		if _, err := e.repo.Ticket().Update(ctx, ticket.WorkspaceID, ticket); err != nil {
			return goerr.Wrap(err, "persist suggested field values", goerr.V("ticket_id", ticket.ID))
		}
	}
	blocks := slackService.BuildReviewBlocks(ctx, e.ticketRef(ticket), comp, ticket.ReporterSlackUserID, schema, ticket.FieldValues)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage review message")
	}
	return nil
}

// finalizeAbort marks the ticket as triaged in the abort path. Done and
// aborted both flip ticket.Triaged → true; the distinction lives in the
// TicketHistory action and reason. This keeps Entry-1 / Entry-2 idempotency
// rules trivial: once Triaged is true, no further triage runs occur.
func (e *PlanExecutor) finalizeAbort(ctx context.Context, ticket *model.Ticket, reason string) error {
	history := &model.TicketHistory{
		Action:    "triage_aborted",
		ChangedBy: ticket.ReporterSlackUserID,
	}
	// Encode the reason in the new status field so List/inspection surfaces it.
	// TicketHistory has no free-form reason field today, so we lean on Action.
	history.Action = "triage_aborted: " + truncate(reason, 240)

	if err := e.repo.Ticket().FinalizeTriage(ctx, ticket.WorkspaceID, ticket.ID, nil, history); err != nil {
		return goerr.Wrap(err, "finalize triage abort in repository", goerr.V("ticket_id", ticket.ID))
	}

	blocks := slackService.BuildAbortedBlocks(ctx, e.ticketRef(ticket), reason)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage abort message")
	}
	return nil
}

// mergeSuggestedFields converts the planner's SuggestedFields map (typed by
// JSON: string / number / []any) into ticket.FieldValues entries, leaving
// any value the operator already supplied untouched. Returns true when at
// least one field was added so the caller knows whether to call Update.
func mergeSuggestedFields(ticket *model.Ticket, comp *model.Complete, schema *domainConfig.FieldSchema) bool {
	if comp == nil || len(comp.SuggestedFields) == 0 || schema == nil {
		return false
	}
	if ticket.FieldValues == nil {
		ticket.FieldValues = make(map[string]model.FieldValue, len(comp.SuggestedFields))
	}
	changed := false
	for _, f := range schema.Fields {
		if _, alreadySet := ticket.FieldValues[f.ID]; alreadySet {
			continue
		}
		raw, ok := comp.SuggestedFields[f.ID]
		if !ok {
			continue
		}
		val, ok := normalizeSuggestedFieldValue(f, raw)
		if !ok {
			continue
		}
		ticket.FieldValues[f.ID] = model.FieldValue{
			FieldID: types.FieldID(f.ID),
			Type:    f.Type,
			Value:   val,
		}
		changed = true
	}
	return changed
}

// normalizeSuggestedFieldValue coerces the JSON-decoded value into the in-Go
// representation that matches the field type. select/multi-select are kept
// as their raw option ids (string or []string); the FieldDefinition's
// declared option list is the source of truth for labels at render time.
func normalizeSuggestedFieldValue(f domainConfig.FieldDefinition, raw any) (any, bool) {
	switch f.Type {
	case types.FieldTypeText, types.FieldTypeURL, types.FieldTypeSelect,
		types.FieldTypeUser, types.FieldTypeDate:
		s, ok := raw.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, false
		}
		return s, true
	case types.FieldTypeNumber:
		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
		return nil, false
	case types.FieldTypeMultiSelect, types.FieldTypeMultiUser:
		switch v := raw.(type) {
		case []string:
			if len(v) == 0 {
				return nil, false
			}
			return v, true
		case []any:
			out := make([]string, 0, len(v))
			for _, e := range v {
				s, ok := e.(string)
				if !ok || strings.TrimSpace(s) == "" {
					continue
				}
				out = append(out, s)
			}
			if len(out) == 0 {
				return nil, false
			}
			return out, true
		}
		return nil, false
	}
	return nil, false
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// buildAskBlocks is split out so executor.go and usecase.go can share it.
// baseURL feeds the shared ticket badge so the rendered message stays
// self-identifying even outside the originating thread context.
func buildAskBlocks(ctx context.Context, baseURL string, ticket *model.Ticket, plan *model.TriagePlan) []slackgo.Block {
	header := plan.Message
	if strings.TrimSpace(header) == "" {
		header = "..."
	}
	return slackService.BuildAskBlocks(ctx, ticketRefFromTicket(baseURL, ticket), plan.Ask, header)
}
