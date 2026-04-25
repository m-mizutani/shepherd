package prompt

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed mention.md
var mentionTemplateSource string

var mentionTemplate = template.Must(template.New("mention").Parse(mentionTemplateSource))

// MentionInput is the data passed to the mention prompt template.
type MentionInput struct {
	Title          string
	Description    string
	InitialMessage string
	Comments       []MentionComment
	MentionAuthor  string
	Mention        string
}

// MentionComment is one prior reply in the ticket thread.
type MentionComment struct {
	Author string
	Role   string
	Body   string
}

// RenderMention executes the mention prompt template with the given input.
func RenderMention(in MentionInput) (string, error) {
	var buf strings.Builder
	if err := mentionTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "failed to execute mention template")
	}
	return buf.String(), nil
}
