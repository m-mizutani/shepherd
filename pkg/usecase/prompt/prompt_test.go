package prompt_test

import (
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/usecase/prompt"
)

func TestRenderMention_FullInput(t *testing.T) {
	got, err := prompt.RenderMention(prompt.MentionInput{
		Title:          "Login fails on Safari",
		Description:    "Users on Safari 17 see a blank page after sign-in.",
		InitialMessage: "Hi, login is broken on Safari.",
		Comments: []prompt.MentionComment{
			{Author: "alice", Role: "reporter", Body: "Confirmed on Safari 17.4."},
			{Author: "bob", Role: "engineer", Body: "Looks like a CSP issue."},
		},
		MentionAuthor: "carol",
		Mention:       "Any update on this?",
	})
	gt.NoError(t, err)

	for _, want := range []string{
		"Login fails on Safari",
		"Users on Safari 17",
		"Hi, login is broken on Safari.",
		"alice",
		"reporter",
		"Confirmed on Safari 17.4.",
		"bob",
		"engineer",
		"Looks like a CSP issue.",
		"Latest mention from carol",
		"Any update on this?",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered prompt missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderMention_EmptyComments(t *testing.T) {
	got, err := prompt.RenderMention(prompt.MentionInput{
		Title:         "First report",
		MentionAuthor: "dave",
		Mention:       "ping",
	})
	gt.NoError(t, err)
	gt.S(t, got).Contains("First report")
	gt.S(t, got).Contains("(no prior replies)")
	gt.S(t, got).Contains("Latest mention from dave")
	gt.S(t, got).Contains("ping")
}

func TestRenderMention_OmitsDescriptionWhenEmpty(t *testing.T) {
	got, err := prompt.RenderMention(prompt.MentionInput{
		Title:         "Bare ticket",
		MentionAuthor: "eve",
		Mention:       "hey",
	})
	gt.NoError(t, err)
	if strings.Contains(got, "- Description:") {
		t.Errorf("expected description block to be omitted, got:\n%s", got)
	}
	if strings.Contains(got, "- Initial message:") {
		t.Errorf("expected initial message block to be omitted, got:\n%s", got)
	}
}
