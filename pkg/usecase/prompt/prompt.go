package prompt

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed system.md
var systemTemplateSource string

//go:embed mention.md
var mentionTemplateSource string

var (
	systemTemplate  = template.Must(template.New("system").Parse(systemTemplateSource))
	mentionTemplate = template.Must(template.New("mention").Parse(mentionTemplateSource))
)

// SystemInput is the data for the system prompt template. It carries the
// static ticket context that does not change between turns inside the same
// thread, and is injected once per agent.Execute call via
// gollem.WithSystemPrompt.
type SystemInput struct {
	Title          string
	Description    string
	InitialMessage string
}

// MentionInput is the data for the per-turn user prompt template. It carries
// only the latest mention, since the prior conversation lives in the gollem
// history layer.
type MentionInput struct {
	MentionAuthor string
	Mention       string
}

// RenderSystem renders the system prompt for the agent.
func RenderSystem(in SystemInput) (string, error) {
	var buf strings.Builder
	if err := systemTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute system template")
	}
	return buf.String(), nil
}

// RenderMention renders the user prompt for the latest mention.
func RenderMention(in MentionInput) (string, error) {
	var buf strings.Builder
	if err := mentionTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute mention template")
	}
	return buf.String(), nil
}
