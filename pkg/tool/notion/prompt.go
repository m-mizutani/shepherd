package notion

import (
	"context"
	_ "embed"
	"strings"
	"text/template"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

//go:embed prompt.md
var promptTemplateSource string

var promptTemplate = template.Must(template.New("notion_provider_prompt").Parse(promptTemplateSource))

// promptInput is the data shape passed into prompt.md. Sources are flattened
// to the few fields the planner actually reads (ID/Title/ObjectType plus the
// owner-supplied description). Notion-specific bits like ObjectID stay out
// because surfacing raw 32-hex IDs to the planner adds noise without helping.
type promptInput struct {
	Sources []promptSource
}

type promptSource struct {
	ID              string
	Title           string
	ObjectType      string
	UserDescription string
}

// renderPrompt produces the Notion provider's narrative for the planner
// system prompt, including the workspace's currently registered sources.
// When the factory has no client (Init skipped because token was unset)
// the function returns "" so the planner sees nothing for this provider.
func (f *Factory) renderPrompt(ctx context.Context, ws types.WorkspaceID) (string, error) {
	if f.client == nil || f.sourceRepo == nil {
		return "", nil
	}
	srcs, err := f.sourceRepo.ListByProvider(ctx, ws, types.SourceProviderNotion)
	if err != nil {
		return "", goerr.Wrap(err, "list notion sources for provider prompt",
			goerr.V("workspace_id", string(ws)))
	}
	return renderPromptFromSources(srcs)
}

// renderPromptFromSources is the pure-function half of renderPrompt: takes
// the source slice already fetched from the repository and returns the
// rendered markdown. Split out so unit tests can exercise the template
// without standing up a Notion client.
func renderPromptFromSources(srcs []*model.Source) (string, error) {
	in := promptInput{Sources: toPromptSources(srcs)}
	var buf strings.Builder
	if err := promptTemplate.Execute(&buf, in); err != nil {
		return "", goerr.Wrap(err, "execute notion provider prompt template")
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func toPromptSources(srcs []*model.Source) []promptSource {
	out := make([]promptSource, 0, len(srcs))
	for _, s := range srcs {
		if s == nil || s.Notion == nil {
			continue
		}
		out = append(out, promptSource{
			ID:              string(s.ID),
			Title:           s.Notion.Title,
			ObjectType:      string(s.Notion.ObjectType),
			UserDescription: s.Description,
		})
	}
	return out
}
