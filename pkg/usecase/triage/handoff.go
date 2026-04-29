package triage

import (
	"context"
	"fmt"
	"strings"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
)

// generateHandoffMessage asks the LLM for a short, friendly Slack message
// addressed to the assignee, summarising the confirmed triage and asking
// them to take it from here. Returns the localised fallback when the LLM is
// not configured or the call fails — never returns an error to the caller,
// since a missing hand-off message must not block ticket finalisation.
func (e *PlanExecutor) generateHandoffMessage(ctx context.Context, comp *model.Complete) string {
	loc := i18n.From(ctx)

	mention := assigneeMentionText(comp)
	fallback := loc.T(i18n.MsgTriageReviewHandoffFallback, "user", mention)

	if e.llm == nil {
		return fallback
	}

	prompt := buildHandoffPrompt(comp, mention)
	agent := gollem.New(e.llm,
		gollem.WithSystemPrompt(prompt.system),
		gollem.WithContentType(gollem.ContentTypeText),
	)
	resp, err := agent.Execute(ctx, gollem.Text(prompt.user))
	if err != nil || resp == nil || len(resp.Texts) == 0 {
		return fallback
	}
	out := strings.TrimSpace(strings.Join(resp.Texts, ""))
	if out == "" {
		return fallback
	}
	// Ensure the assignee mention is present even if the LLM forgot it, so a
	// notification fires.
	if !strings.Contains(out, mention) {
		out = mention + " " + out
	}
	return out
}

func assigneeMentionText(comp *model.Complete) string {
	if comp == nil {
		return ""
	}
	if comp.Assignee.Kind == types.AssigneeAssigned && len(comp.Assignee.UserIDs) > 0 {
		mentions := make([]string, 0, len(comp.Assignee.UserIDs))
		for _, id := range comp.Assignee.UserIDs {
			if id == "" {
				continue
			}
			mentions = append(mentions, fmt.Sprintf("<@%s>", string(id)))
		}
		if len(mentions) > 0 {
			return strings.Join(mentions, " ")
		}
	}
	return "@channel"
}

type handoffPrompt struct {
	system string
	user   string
}

func buildHandoffPrompt(comp *model.Complete, mention string) handoffPrompt {
	system := strings.Join([]string{
		"You write a single short Slack message that hands a triaged ticket off to the chosen assignee.",
		"Match the language of the supplied description (Japanese description → Japanese reply, English → English).",
		"Constraints:",
		"- 1 to 2 sentences total.",
		"- Start with the assignee mention exactly as supplied (do not modify it).",
		"- Politely ask them to take the ticket from here, optionally referencing the most important point of the description.",
		"- Plain text only. No bullet lists, no markdown headings, no preamble like 'Here is the message:'.",
	}, "\n")

	var b strings.Builder
	fmt.Fprintf(&b, "Mention: %s\n", mention)
	if comp != nil {
		if comp.Title != "" {
			fmt.Fprintf(&b, "Title: %s\n", comp.Title)
		}
		if comp.Description != "" {
			fmt.Fprintf(&b, "Description: %s\n", comp.Description)
		}
		if comp.Assignee.Reasoning != "" {
			fmt.Fprintf(&b, "Why this assignee: %s\n", comp.Assignee.Reasoning)
		}
	}
	b.WriteString("\nWrite the hand-off message now.")

	return handoffPrompt{system: system, user: b.String()}
}

