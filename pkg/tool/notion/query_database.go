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

const (
	queryDefaultLimit = 25
	queryMaxLimit     = 100
	// queryBodyCap bounds the per-call cost of include_body=true.
	queryBodyCap = 10
)

type databaseQuerier interface {
	QueryDatabase(ctx context.Context, dbID string, opts notionsvc.QueryDatabaseOptions) (*notionsvc.QueryDatabaseResult, error)
	GetPageMarkdown(ctx context.Context, pageID string) (*notionsvc.PageMarkdown, error)
}

type queryDatabaseTool struct {
	client databaseQuerier
	guard  authorizer
}

func newQueryDatabaseTool(client databaseQuerier, guard authorizer) gollem.Tool {
	return &queryDatabaseTool{client: client, guard: guard}
}

func (t *queryDatabaseTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "notion_query_database",
		Description: "Query rows of a Notion database registered as a Source. " +
			"Args: `database_id` (required, ID or URL); optional `filter` and `sorts` (Notion API objects, passed through verbatim); optional `limit` (default 25, max 100); optional `include_body` (bool) to inline each row's page body as markdown (capped at 10 bodies per call). " +
			"Returns `{ rows: [{id, title, url, markdown?}], count, has_more, next_cursor }`.",
		Parameters: map[string]*gollem.Parameter{
			"database_id": {
				Type:        gollem.TypeString,
				Description: "Notion database ID or full URL.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"filter": {
				Type:        gollem.TypeObject,
				Description: "Notion API filter object (passed through verbatim).",
			},
			"sorts": {
				Type:        gollem.TypeArray,
				Description: "Notion API sorts array (passed through verbatim).",
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum rows to return (default 25, max 100).",
			},
			"include_body": {
				Type:        gollem.TypeBoolean,
				Description: "If true, fetch each row's page body as markdown (capped at 10 bodies/call).",
			},
		},
	}
}

func (t *queryDatabaseTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	rawID, err := argsutil.String(args, "database_id", true)
	if err != nil {
		return nil, err
	}
	_, dbID, err := notionsvc.ParseURL(rawID)
	if err != nil {
		// Fall back to raw normalize: ParseURL defaults to page when there's no
		// ?v= — for raw IDs we don't know the type, so try normalizing.
		dbID, err = notionsvc.NormalizeID(rawID)
		if err != nil {
			return nil, goerr.Wrap(err, "invalid notion database id/url")
		}
	}
	limit := clamp.Limit(argsutil.Int(args, "limit"), queryDefaultLimit, queryMaxLimit)
	includeBody := boolArg(args, "include_body")

	walker, err := t.guard.NewWalker(ctx, wsID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to load notion source roots")
	}
	if err := walker.Authorize(ctx, types.NotionObjectDatabase, dbID); err != nil {
		return nil, goerr.Wrap(err, "notion database not in allowed sources",
			goerr.V("database_id", dbID))
	}

	filter, _ := args["filter"].(map[string]any)
	var sorts []map[string]any
	if rawSorts, ok := args["sorts"].([]any); ok {
		for _, item := range rawSorts {
			if m, ok := item.(map[string]any); ok {
				sorts = append(sorts, m)
			}
		}
	}

	res, err := t.client.QueryDatabase(ctx, dbID, notionsvc.QueryDatabaseOptions{
		Filter:   filter,
		Sorts:    sorts,
		PageSize: limit,
	})
	if err != nil {
		return nil, goerr.Wrap(err, "notion_query_database failed")
	}

	rows := make([]map[string]any, 0, len(res.Pages))
	bodiesFetched := 0
	for _, p := range res.Pages {
		row := map[string]any{
			"id":    normalizeOrEmpty(p.ID),
			"title": p.Title,
			"url":   p.URL,
		}
		if includeBody && bodiesFetched < queryBodyCap {
			pm, mdErr := t.client.GetPageMarkdown(ctx, p.ID)
			if mdErr == nil {
				row["markdown"] = pm.Markdown
				bodiesFetched++
			}
		}
		rows = append(rows, row)
	}

	return map[string]any{
		"rows":        rows,
		"count":       len(rows),
		"has_more":    res.HasMore,
		"next_cursor": res.NextCursor,
	}, nil
}
