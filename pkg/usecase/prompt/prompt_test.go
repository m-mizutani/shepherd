package prompt_test

import (
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
)

func TestRenderSystem_FullInput(t *testing.T) {
	got, err := prompt.RenderSystem(prompt.SystemInput{
		Title:          "Login fails on Safari",
		Description:    "Users on Safari 17 see a blank page after sign-in.",
		InitialMessage: "Hi, login is broken on Safari.",
	})
	gt.NoError(t, err)

	for _, want := range []string{
		"Slack assistant",
		"Login fails on Safari",
		"Users on Safari 17",
		"Hi, login is broken on Safari.",
		"conversation history",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered system prompt missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderSystem_OmitsEmptyBlocks(t *testing.T) {
	got, err := prompt.RenderSystem(prompt.SystemInput{
		Title: "Bare ticket",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Bare ticket")
	if strings.Contains(got, "- Description:") {
		t.Errorf("expected description block to be omitted, got:\n%s", got)
	}
	if strings.Contains(got, "- Initial message:") {
		t.Errorf("expected initial message block to be omitted, got:\n%s", got)
	}
}

func TestRenderMention_HappyPath(t *testing.T) {
	got, err := prompt.RenderMention(prompt.MentionInput{
		MentionAuthor: "carol",
		Mention:       "Any update on this?",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("carol")
	gt.S(t, got).Contains("Any update on this?")
	if strings.Contains(got, "Login fails on Safari") {
		t.Errorf("mention prompt must not contain ticket title (that's in system prompt)")
	}
}

func TestRenderTriagePlan_FullInput(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title:          "Sign-in broken on Safari",
		Description:    "Users on Safari 17 see a blank page.",
		InitialMessage: "Hi, login is broken.",
		Reporter:       "U123",
	})
	gt.NoError(t, err)

	for _, want := range []string{
		"propose_investigate",
		"propose_ask",
		"propose_complete",
		"Sign-in broken on Safari",
		"Users on Safari 17",
		"<@U123>",
		"acceptance_criteria",
		"unassigned",
		"user_ids",
		"empty array",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("triage_plan prompt missing %q\n---\n%s", want, got)
		}
	}

	for _, banned := range []string{
		"a single user",
		"a single owner",
		`kind: "assigned"`,
		`kind: "unassigned"`,
		`kind=='assigned'`,
	} {
		if strings.Contains(got, banned) {
			t.Errorf("triage_plan prompt must not reference removed kind discriminator %q\n---\n%s", banned, got)
		}
	}
}

func TestRenderTriagePlan_OmitsEmptyOptionals(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title: "Bare ticket",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Bare ticket")
	if strings.Contains(got, "- Description:") {
		t.Errorf("expected description line omitted, got:\n%s", got)
	}
	if strings.Contains(got, "- Reporter:") {
		t.Errorf("expected reporter line omitted, got:\n%s", got)
	}
	if strings.Contains(got, "\n---\n") {
		t.Errorf("expected no UserGuidance separator when guidance is empty, got:\n%s", got)
	}
}

func TestRenderTriagePlan_AppendsUserGuidanceAfterSeparator(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title:        "Sign-in broken",
		UserGuidance: "Always escalate production outages to the on-call engineer.",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Sign-in broken")
	gt.S(t, got).Contains("Always escalate production outages")

	// The separator must precede the user guidance so that it reads as an
	// independent markdown block, even when the guidance starts with a
	// heading (see TestRenderTriagePlan_UserGuidanceWithLeadingHeading).
	idxSep := strings.Index(got, "\n---\n")
	idxGuidance := strings.Index(got, "Always escalate production outages")
	if idxSep < 0 {
		t.Fatalf("expected UserGuidance separator '\\n---\\n' in output, got:\n%s", got)
	}
	if idxSep > idxGuidance {
		t.Errorf("expected separator to precede guidance, got:\n%s", got)
	}
}

func TestRenderTriagePlan_UserGuidanceWithLeadingHeading(t *testing.T) {
	// A user-supplied guidance starting with a Markdown H1 must not collide
	// with the heading hierarchy of the base template — the separator
	// inserted before it makes the user block a self-contained document.
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title:        "Bare ticket",
		UserGuidance: "# Workspace policy\n\nNever auto-assign tickets without manager approval.",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("# Workspace policy")
	gt.S(t, got).Contains("Never auto-assign tickets")

	// The base template's '## Rules' heading must not directly precede the
	// user's H1 — the separator + blank line gives the H1 room to stand on
	// its own.
	if strings.Contains(got, "## Rules\n# Workspace policy") {
		t.Errorf("user H1 collided with base template heading, got:\n%s", got)
	}
	idxSep := strings.Index(got, "\n---\n")
	idxHeading := strings.Index(got, "# Workspace policy")
	if idxSep < 0 || idxSep > idxHeading {
		t.Errorf("expected separator before user H1, got:\n%s", got)
	}
}

func TestRenderTriagePlan_AutoFillFieldsRendered(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title: "Crash on save",
		AutoFillFields: []prompt.AutoFillField{
			{
				ID:          "severity",
				Name:        "Severity",
				Type:        "select",
				Description: "Triage urgency level",
				Required:    true,
				Options: []prompt.AutoFillOption{
					{ID: "p0", Label: "Sev 0 — outage", Description: "Active or imminent customer impact"},
					{ID: "p1", Label: "Sev 1 — major"},
				},
			},
			{
				ID:   "tags",
				Name: "Tags",
				Type: "multi-select",
				Options: []prompt.AutoFillOption{
					{ID: "frontend", Label: "Frontend"},
				},
			},
		},
	})
	gt.NoError(t, err)
	for _, want := range []string{
		"Auto-fill custom fields",
		"`severity`",
		"Severity",
		"required",
		"Triage urgency level",
		"`p0`",
		"Sev 0 — outage",
		"Active or imminent customer impact",
		"`tags`",
		"multi-select",
		"`frontend`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("auto_fill section missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderTriagePlan_NoAutoFillSectionWhenEmpty(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{Title: "x"})
	gt.NoError(t, err)
	if strings.Contains(got, "Auto-fill custom fields") {
		t.Errorf("auto_fill section should be omitted when no fields are configured, got:\n%s", got)
	}
}

func TestRenderTriagePlan_AvailableToolsRendered(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title: "Sign-in broken",
		AvailableTools: []prompt.ProviderBriefing{
			{
				ID:          "slack",
				Description: "Searches the connected Slack workspace.",
				Tools: []prompt.ToolEntry{
					{Name: "slack_search_messages", Description: "Search Slack messages by query."},
					{Name: "slack_get_thread", Description: "Read a Slack thread by channel + ts."},
				},
			},
			{
				ID:          "notion",
				Description: "Reads Notion content scoped to registered Sources.",
				Tools: []prompt.ToolEntry{
					{Name: "notion_search", Description: "Full-text search inside sources."},
				},
			},
		},
	})
	gt.NoError(t, err)
	for _, want := range []string{
		"Available investigation tools",
		"### slack",
		"Searches the connected Slack workspace.",
		"`slack_search_messages` — Search Slack messages by query.",
		"`slack_get_thread`",
		"### notion",
		"Reads Notion content scoped to registered Sources.",
		"`notion_search` — Full-text search inside sources.",
		"silently dropped",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("triage_plan AvailableTools section missing %q\n---\n%s", want, got)
		}
	}
	if strings.Contains(got, "No investigation tools are enabled") {
		t.Errorf("expected populated tools branch, got fallback message:\n%s", got)
	}
}

func TestRenderTriagePlan_NoToolsFallback(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{Title: "x"})
	gt.NoError(t, err)
	gt.S(t, got).Contains("No investigation tools are enabled for this workspace")
	gt.S(t, got).Contains("Do not call `propose_investigate`")
}

func TestRenderTriagePlan_ProviderWithEmptyDescriptionStillListsTools(t *testing.T) {
	got, err := prompt.RenderTriagePlan(prompt.TriagePlanInput{
		Title: "x",
		AvailableTools: []prompt.ProviderBriefing{
			{
				ID:          "ticket",
				Description: "", // factory.Prompt errored — narrative blanked
				Tools: []prompt.ToolEntry{
					{Name: "ticket_get", Description: "Fetch a ticket."},
				},
			},
		},
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("### ticket")
	gt.S(t, got).Contains("`ticket_get` — Fetch a ticket.")
}

func TestRenderTriageSubtask_RendersCriteria(t *testing.T) {
	got, err := prompt.RenderTriageSubtask(prompt.TriageSubtaskInput{
		Request: "Collect related Slack posts in the last 48h",
		AcceptanceCriteria: []string{
			"Returns at least 3 messages or explicitly states 'no related messages'",
			"Includes channel, timestamp, and excerpt for each message",
		},
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Collect related Slack posts")
	gt.S(t, got).Contains("Returns at least 3 messages")
	gt.S(t, got).Contains("Includes channel, timestamp")
	gt.S(t, got).Contains("triage investigation agent")
}

func TestRenderTriageSubtask_EmptyCriteriaStillRenders(t *testing.T) {
	got, err := prompt.RenderTriageSubtask(prompt.TriageSubtaskInput{
		Request:            "Identify owner of the affected service",
		AcceptanceCriteria: nil,
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("Identify owner of the affected service")
}

func TestRenderConclusion_FullInput(t *testing.T) {
	got, err := prompt.RenderConclusion(prompt.ConclusionInput{
		Title:          "Login fails on Safari",
		Description:    "Users on Safari 17 see a blank page after sign-in.",
		InitialMessage: "Hi, login is broken on Safari.",
		Comments: []prompt.ConclusionComment{
			{Author: "U_REPORTER", Body: "Repro on Safari 17, Chrome works."},
			{Body: "Triage: investigating CSP violations."},
			{Author: "U_OWNER", Body: "Patch landed in #1234, please verify."},
		},
		Language: "English",
	})
	gt.NoError(t, err)

	for _, want := range []string{
		"Login fails on Safari",
		"Users on Safari 17",
		"Hi, login is broken on Safari.",
		"<@U_REPORTER>",
		"Repro on Safari 17, Chrome works.",
		"Triage: investigating CSP violations.",
		"<@U_OWNER>",
		"Patch landed in #1234",
		"\"conclusion\":",
		// New retrospective shape
		"do **not** restate",
		"Essence of the problem",
		"How it was resolved",
		"Process retrospective",
		"Requester",
		"Responder",
		"AI / automation",
		"at most 2 to 3 short sections",
		"common subset of Slack mrkdwn and standard Markdown",
		"Do not include any emoji",
		"in English",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("conclusion prompt missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderConclusion_NoCommentsFallback(t *testing.T) {
	got, err := prompt.RenderConclusion(prompt.ConclusionInput{
		Title:    "Empty thread",
		Language: "Japanese",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("No thread messages were captured")
	if strings.Contains(got, "- Description:") {
		t.Errorf("expected description block to be omitted for empty input, got:\n%s", got)
	}
	gt.S(t, got).Contains("in Japanese")
}
