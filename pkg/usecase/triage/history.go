// Package triage implements the LLM-driven ticket triage agent.
//
// The triage state is materialised through two channels:
//
//   - ticket.Triaged on the Firestore ticket: the only persistent flag triage
//     adds to the ticket itself.
//   - gollem agent history at session "{workspace}/{ticket}/plan": the canonical
//     record of every LLM turn (tool calls, responses, user-side updates).
//
// This file holds the helpers that read and write that history without
// involving the gollem agent driver itself: helpers for appending user
// messages, recovering the most recent TriagePlan, deciding whether the
// agent is currently waiting on a Slack submit, and counting iterations
// for the cap check.
package triage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

const (
	// proposeInvestigateToolName is the gollem tool name the LLM calls to
	// schedule a parallel investigation.
	proposeInvestigateToolName = "propose_investigate"
	// proposeAskToolName is the gollem tool name the LLM calls to ask the
	// reporter follow-up questions.
	proposeAskToolName = "propose_ask"
	// proposeCompleteToolName is the gollem tool name the LLM calls to finish
	// triage with a hand-off summary.
	proposeCompleteToolName = "propose_complete"
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
// most recent propose_* tool call and decodes it into a TriagePlan. Returns
// (nil, nil) when there is no plan history yet or the trailing message is not
// a propose_* tool call.
func loadLatestTriagePlan(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (*model.TriagePlan, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return nil, goerr.Wrap(err, "load plan history")
	}
	if h == nil {
		return nil, nil
	}
	tc := findLatestProposeToolCall(h)
	if tc == nil {
		return nil, nil
	}
	return decodeTriagePlanFromToolCall(tc)
}

// isWaitingUserSubmit reports true when the latest propose_* tool call is a
// propose_ask AND no user-role message has been appended after it. That
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

	// Walk forward; remember the last propose_* tool call name and whether
	// any user-role message appeared after it.
	var lastProposeName string
	userAfterPropose := false
	for _, msg := range h.Messages {
		if msg.Role == gollem.RoleUser && lastProposeName != "" {
			userAfterPropose = true
		}
		for _, c := range msg.Contents {
			if c.Type != gollem.MessageContentTypeToolCall {
				continue
			}
			tc, err := c.GetToolCallContent()
			if err != nil {
				continue
			}
			if isProposeToolName(tc.Name) {
				lastProposeName = tc.Name
				userAfterPropose = false
			}
		}
	}
	return lastProposeName == proposeAskToolName && !userAfterPropose, nil
}

// countToolCalls returns the number of propose_* tool calls already recorded
// in the plan history. The plan executor uses this to enforce its iteration
// cap without persisting a separate counter.
func countToolCalls(ctx context.Context, repo gollem.HistoryRepository, workspaceID types.WorkspaceID, ticketID types.TicketID) (int, error) {
	h, err := repo.Load(ctx, planSessionID(workspaceID, ticketID))
	if err != nil {
		return 0, goerr.Wrap(err, "load plan history")
	}
	if h == nil {
		return 0, nil
	}
	count := 0
	for _, msg := range h.Messages {
		for _, c := range msg.Contents {
			if c.Type != gollem.MessageContentTypeToolCall {
				continue
			}
			tc, err := c.GetToolCallContent()
			if err != nil {
				continue
			}
			if isProposeToolName(tc.Name) {
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

func isProposeToolName(name string) bool {
	switch name {
	case proposeInvestigateToolName, proposeAskToolName, proposeCompleteToolName:
		return true
	default:
		return false
	}
}

func findLatestProposeToolCall(h *gollem.History) *gollem.ToolCallContent {
	for i := len(h.Messages) - 1; i >= 0; i-- {
		msg := h.Messages[i]
		for j := len(msg.Contents) - 1; j >= 0; j-- {
			c := msg.Contents[j]
			if c.Type != gollem.MessageContentTypeToolCall {
				continue
			}
			tc, err := c.GetToolCallContent()
			if err != nil {
				continue
			}
			if isProposeToolName(tc.Name) {
				return tc
			}
		}
	}
	return nil
}

// decodeTriagePlanFromToolCall converts a captured propose_* tool call (as
// stored in the session history) into a TriagePlan.
func decodeTriagePlanFromToolCall(tc *gollem.ToolCallContent) (*model.TriagePlan, error) {
	if tc == nil {
		return nil, goerr.New("nil tool call")
	}
	return decodePlan(tc.Name, tc.Arguments)
}

// decodePlanFromFunctionCall converts a fresh function call returned by
// Session.Generate into a TriagePlan. Same shape as decodeTriagePlanFromToolCall
// but reads the FunctionCall variant of the same payload.
func decodePlanFromFunctionCall(fc *gollem.FunctionCall) (*model.TriagePlan, error) {
	if fc == nil {
		return nil, goerr.New("nil function call")
	}
	return decodePlan(fc.Name, fc.Arguments)
}

func decodePlan(name string, args map[string]any) (*model.TriagePlan, error) {
	plan := &model.TriagePlan{}
	if msg, ok := args["message"].(string); ok {
		plan.Message = msg
	}

	switch name {
	case proposeInvestigateToolName:
		plan.Kind = types.PlanInvestigate
		var inv model.Investigate
		if err := remarshal(args, &inv); err != nil {
			return nil, goerr.Wrap(err, "decode propose_investigate args")
		}
		plan.Investigate = &inv
	case proposeAskToolName:
		plan.Kind = types.PlanAsk
		var ask model.Ask
		if err := remarshal(args, &ask); err != nil {
			return nil, goerr.Wrap(err, "decode propose_ask args")
		}
		plan.Ask = &ask
	case proposeCompleteToolName:
		plan.Kind = types.PlanComplete
		var comp model.Complete
		if err := remarshal(args, &comp); err != nil {
			return nil, goerr.Wrap(err, "decode propose_complete args")
		}
		plan.Complete = &comp
	default:
		return nil, goerr.New("unknown propose tool", goerr.V("name", name))
	}
	if err := plan.Validate(); err != nil {
		return nil, goerr.Wrap(err, "invalid plan from tool call")
	}
	return plan, nil
}

// remarshal copies fields between a free-form map and a typed struct using
// JSON as the wire format. The propose_* tool argument names are chosen to
// match the JSON-friendly form of the model structs (see field tags below
// when introduced), so this round-trip yields a clean conversion.
func remarshal(src map[string]any, dst any) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	return dec.Decode(dst)
}
