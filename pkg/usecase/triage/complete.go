package triage

import (
	"context"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
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
	if titleChanged || descChanged {
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
// require_review = false (planner converges → finalize immediately).
func (e *PlanExecutor) finalizeCompleteAndAnnounce(ctx context.Context, ticket *model.Ticket, comp *model.Complete) error {
	if err := e.finalizeComplete(ctx, ticket, comp); err != nil {
		return err
	}
	blocks := slackService.BuildCompleteBlocks(ctx, comp)
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
	blocks := slackService.BuildReviewBlocks(ctx, ticket.ID, comp, ticket.ReporterSlackUserID)
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

	blocks := slackService.BuildAbortedBlocks(ctx, reason)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage abort message")
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// buildAskBlocks is split out so executor.go and usecase.go can share it.
func buildAskBlocks(ctx context.Context, ticket *model.Ticket, plan *model.TriagePlan) []slackgo.Block {
	header := plan.Message
	if strings.TrimSpace(header) == "" {
		header = "..."
	}
	return slackService.BuildAskBlocks(ctx, ticket.ID, plan.Ask, header)
}
