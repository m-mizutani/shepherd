package notion

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
)

// searcher is the slice of *notion.Client this tool uses, so tests can fake.
type searcher interface {
	Search(ctx context.Context, query string, opts notionsvc.SearchOptions) ([]*notionsvc.SearchHit, error)
}

// authorizer abstracts the NotionGuard.Authorize signature.
type authorizer interface {
	Authorize(ctx context.Context, ws types.WorkspaceID, t types.NotionObjectType, id string) error
}

const (
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

type searchTool struct {
	client searcher
	guard  authorizer
}

func newSearchTool(client searcher, guard authorizer) gollem.Tool {
	return &searchTool{client: client, guard: guard}
}

func (t *searchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "notion_search",
		Description: "Search Notion content within this workspace's allowed sources " +
			"(registered pages and databases). Returns matches scoped to those roots.",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Free-text search query.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"object_type": {
				Type:        gollem.TypeString,
				Description: "Restrict search to 'page' or 'database'. Defaults to any.",
				Enum:        []string{"page", "database", "any"},
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum results to return (default 20, max 50).",
			},
		},
	}
}

func (t *searchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	query, err := argsutil.String(args, "query", true)
	if err != nil {
		return nil, err
	}
	limit := clamp.Limit(argsutil.Int(args, "limit"), searchDefaultLimit, searchMaxLimit)
	objType, _ := argsutil.String(args, "object_type", false)
	if objType == "any" {
		objType = ""
	}

	hits, err := t.client.Search(ctx, query, notionsvc.SearchOptions{ObjectType: objType, PageSize: searchMaxLimit})
	if err != nil {
		return nil, goerr.Wrap(err, "notion_search failed")
	}

	allowed := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		if err := t.guard.Authorize(ctx, wsID, h.ObjectType, normalizeOrEmpty(h.ID)); err != nil {
			continue
		}
		allowed = append(allowed, map[string]any{
			"id":    normalizeOrEmpty(h.ID),
			"type":  string(h.ObjectType),
			"title": h.Title,
			"url":   h.URL,
		})
		if len(allowed) >= limit {
			break
		}
	}
	return map[string]any{
		"matches": allowed,
		"count":   len(allowed),
	}, nil
}

func normalizeOrEmpty(id string) string {
	out, err := notionsvc.NormalizeID(id)
	if err != nil {
		return id
	}
	return out
}

func workspaceFromCtx(ctx context.Context) (types.WorkspaceID, error) {
	id, ok := types.WorkspaceFromContext(ctx)
	if !ok || id == "" {
		return "", goerr.New("no active workspace bound to context")
	}
	return id, nil
}
