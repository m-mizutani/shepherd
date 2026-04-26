package notion

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// listSourcesTool surfaces the workspace's registered Notion Sources to the
// LLM so it can pick a target before issuing notion_search /
// notion_query_database. Description is the user-supplied free text.
type listSourcesTool struct {
	repo interfaces.SourceRepository
}

func newListSourcesTool(repo interfaces.SourceRepository) gollem.Tool {
	return &listSourcesTool{repo: repo}
}

func (t *listSourcesTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "notion_list_sources",
		Description: "List Notion pages/databases registered as Sources for this workspace. " +
			"Each entry has a user-supplied description explaining what it contains — " +
			"call this first to decide which Source to target with notion_search or " +
			"notion_query_database.",
		Parameters: map[string]*gollem.Parameter{},
	}
}

func (t *listSourcesTool) Run(ctx context.Context, _ map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	srcs, err := t.repo.ListByProvider(ctx, wsID, types.SourceProviderNotion)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list notion sources")
	}
	out := make([]map[string]any, 0, len(srcs))
	for _, s := range srcs {
		entry := map[string]any{
			"id":          string(s.ID),
			"description": s.Description,
		}
		if s.Notion != nil {
			entry["object_type"] = string(s.Notion.ObjectType)
			entry["object_id"] = s.Notion.ObjectID
			entry["title"] = s.Notion.Title
			entry["url"] = s.Notion.URL
		}
		out = append(out, entry)
	}
	return map[string]any{
		"sources": out,
		"count":   len(out),
	}, nil
}
