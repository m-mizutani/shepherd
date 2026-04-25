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
