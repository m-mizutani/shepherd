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

// finalizeComplete records ticket.Triaged=true (with optional assignee
// update) atomically with a TicketHistory entry, then posts the hand-off
// summary to the ticket's Slack thread. Slack post failures do not roll back
// the DB write — the ticket is still considered triaged, and the failure is
// surfaced to the caller (which routes through errutil.Handle).
func (e *PlanExecutor) finalizeComplete(ctx context.Context, ticket *model.Ticket, plan *model.TriagePlan) error {
	if plan.Complete == nil {
		return goerr.New("plan kind complete without payload")
	}
	comp := plan.Complete

	var assignee *types.SlackUserID
	if comp.Assignee.Kind == types.AssigneeAssigned && comp.Assignee.UserID != nil {
		assignee = comp.Assignee.UserID
	}

	history := &model.TicketHistory{
		Action:    "triage_completed",
		ChangedBy: ticket.ReporterSlackUserID, // best-effort: the bot has no Slack user id of its own.
	}

	if err := e.repo.Ticket().FinalizeTriage(ctx, ticket.WorkspaceID, ticket.ID, assignee, history); err != nil {
		return goerr.Wrap(err, "finalize triage in repository", goerr.V("ticket_id", ticket.ID))
	}

	blocks := slackService.BuildCompleteBlocks(ctx, comp)
	if _, err := e.slack.PostThreadBlocks(ctx, string(ticket.SlackChannelID), string(ticket.SlackThreadTS), blocks); err != nil {
		return goerr.Wrap(err, "post triage complete message")
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
