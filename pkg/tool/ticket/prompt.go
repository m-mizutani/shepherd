package ticket

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed prompt.md
var promptTemplateSource string

var promptTemplate = template.Must(template.New("ticket_provider_prompt").Parse(promptTemplateSource))

// renderPrompt produces the ticket provider's narrative for the planner
// system prompt.
func renderPrompt() (string, error) {
	var buf strings.Builder
	if err := promptTemplate.Execute(&buf, nil); err != nil {
		return "", goerr.Wrap(err, "execute ticket provider prompt template")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
