package slack

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed prompt.md
var promptTemplateSource string

var promptTemplate = template.Must(template.New("slack_provider_prompt").Parse(promptTemplateSource))

// renderPrompt produces the slack provider's narrative for the planner
// system prompt. The template currently has no dynamic data, but going
// through text/template keeps the file format consistent with the other
// tool packages (.claude/rules/prompts.md).
func renderPrompt() (string, error) {
	var buf strings.Builder
	if err := promptTemplate.Execute(&buf, nil); err != nil {
		return "", goerr.Wrap(err, "execute slack provider prompt template")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
