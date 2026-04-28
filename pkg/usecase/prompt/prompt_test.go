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
	} {
		if !strings.Contains(got, want) {
			t.Errorf("triage_plan prompt missing %q\n---\n%s", want, got)
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
