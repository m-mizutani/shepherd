// Package triage implements the LLM-driven ticket triage agent.
//
// The triage state is materialised through two channels:
//
//   - ticket.Triaged on the Firestore ticket: the only persistent flag triage
//     adds to the ticket itself.
//   - gollem agent history at session "{workspace}/{ticket}/plan": the canonical
//     record of every LLM turn (assistant JSON plans, user-side updates).
//
// This file holds the helpers that read and write that history without
// involving the gollem agent driver itself: helpers for appending user
// messages, recovering the most recent TriagePlan, deciding whether the
// agent is currently waiting on a Slack submit, and counting iterations
// for the cap check.
package triage

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// planSessionID returns the gollem history session identifier for a ticket's
// plan-level conversation.
func planSessionID(workspaceID types.WorkspaceID, ticketID types.TicketID) string {
	return fmt.Sprintf("%s/%s/plan", workspaceID, ticketID)
}

// subtaskSessionID returns the gollem history session identifier for a child
// investigation agent.
func subtaskSessionID(workspaceID types.WorkspaceID, ticketID types.TicketID, subtaskID types.SubtaskID) string {
	return fmt.Sprintf("%s/%s/sub/%s", workspaceID, ticketID, subtaskID)
}

// appendUserMessage appends a user-role text message to the plan-level history
// for the given ticket. This is how triage feeds investigation results and
// reporter answers back into the LLM context: the next llmPlan call sees
// these messages naturally as conversation history.
func appendUserMessage(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID, text string) error {
	sid := planSessionID(workspaceID, ticketID)
	h, err := repo.Load(ctx, sid)
	if err != nil {
		return goerr.Wrap(err, "load plan history", goerr.V("session", sid))
	}
	if h == nil {
		h = &gollem.History{Version: gollem.HistoryVersion}
	}

	content, err := gollem.NewTextContent(text)
	if err != nil {
		return goerr.Wrap(err, "build text content")
	}
	h.Messages = append(h.Messages, gollem.Message{
		Role:     gollem.RoleUser,
		Contents: []gollem.MessageContent{content},
	})
	if err := repo.Save(ctx, sid, h); err != nil {
		return goerr.Wrap(err, "save plan history", goerr.V("session", sid))
	}
	return nil
}

// loadLatestTriagePlan walks the plan-level history backwards looking for the
// most recent assistant message and decodes its text payload as a structured
// TriagePlan (the LLM produces it under triagePlanSchema). Returns (nil, nil)
// when there is no plan history yet.
func loadLatestTriagePlan(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (*model.TriagePlan, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return nil, goerr.Wrap(err, "load plan history")
	}
	if h == nil {
		return nil, nil
	}
	raw := latestAssistantText(h)
	if raw == "" {
		return nil, nil
	}
	plan, err := decodePlanFromJSON(raw)
	if err != nil {
		return nil, goerr.Wrap(err, "decode latest plan")
	}
	return plan, nil
}

// isWaitingUserSubmit reports true when the latest assistant plan is a
// kind=ask AND no user-role message has been appended after it. That
// combination is exactly the condition for "the reporter has been shown a
// question form and we're waiting on their submit".
func isWaitingUserSubmit(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (bool, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return false, goerr.Wrap(err, "load plan history")
	}
	if h == nil {
		return false, nil
	}

	// Walk forward; remember the last assistant plan kind and whether any
	// user-role message has appeared after it.
	var lastKind types.PlanKind
	seenAssistant := false
	userAfterAssistant := false
	for _, msg := range h.Messages {
		switch msg.Role {
		case gollem.RoleAssistant:
			if text := joinAssistantText(msg); text != "" {
				if plan, err := decodePlanFromJSON(text); err == nil {
					lastKind = plan.Kind
					seenAssistant = true
					userAfterAssistant = false
				}
			}
		case gollem.RoleUser:
			if seenAssistant {
				userAfterAssistant = true
			}
		}
	}
	return seenAssistant && lastKind == types.PlanAsk && !userAfterAssistant, nil
}

// countPlannerTurns returns how many assistant plan messages the planner has
// already produced for this ticket. The plan executor uses this to enforce
// its iteration cap without persisting a separate counter.
func countPlannerTurns(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (int, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return 0, goerr.Wrap(err, "load plan history")
	}
	if h == nil {
		return 0, nil
	}
	count := 0
	for _, msg := range h.Messages {
		if msg.Role != gollem.RoleAssistant {
			continue
		}
		if text := joinAssistantText(msg); text != "" {
			if _, err := decodePlanFromJSON(text); err == nil {
				count++
			}
		}
	}
	return count, nil
}

// hasPlanHistory reports whether any plan-level history exists yet for this
// ticket. Useful to distinguish "not started" from "in progress".
func hasPlanHistory(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (bool, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return false, goerr.Wrap(err, "load plan history")
	}
	return h != nil && len(h.Messages) > 0, nil
}

func latestAssistantText(h *gollem.History) string {
	for i := len(h.Messages) - 1; i >= 0; i-- {
		if h.Messages[i].Role != gollem.RoleAssistant {
			continue
		}
		if text := joinAssistantText(h.Messages[i]); text != "" {
			return text
		}
	}
	return ""
}

func joinAssistantText(msg gollem.Message) string {
	var b strings.Builder
	for _, c := range msg.Contents {
		if c.Type != gollem.MessageContentTypeText {
			continue
		}
		tc, err := c.GetTextContent()
		if err != nil || tc == nil {
			continue
		}
		b.WriteString(tc.Text)
	}
	return b.String()
}

