package meta

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
)

//go:embed prompt.md
var promptTemplateSource string

var promptTemplate = template.Must(template.New("meta_provider_prompt").Parse(promptTemplateSource))

// renderPrompt produces the meta provider's narrative for the planner
// system prompt.
func renderPrompt() (string, error) {
	var buf strings.Builder
	if err := promptTemplate.Execute(&buf, nil); err != nil {
		return "", goerr.Wrap(err, "execute meta provider prompt template")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}
