package notion

import (
	"context"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	notionsvc "github.com/m-mizutani/shepherd/pkg/service/notion"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
)

// pageReader is the slice of *notion.Client get_page uses.
type pageReader interface {
	GetPageMarkdown(ctx context.Context, pageID string) (*notionsvc.PageMarkdown, error)
	RetrievePage(ctx context.Context, pageID string) (*notionsvc.PageMeta, error)
}

const (
	getPageDefaultDepth = 2
	getPageMaxDepth     = 4
	getPageDefaultPages = 20
	getPageMaxPages     = 50
)

type getPageTool struct {
	client pageReader
	guard  authorizer
}

func newGetPageTool(client pageReader, guard authorizer) gollem.Tool {
	return &getPageTool{client: client, guard: guard}
}

func (t *getPageTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "notion_get_page",
		Description: "Fetch a Notion page as Markdown. With recursive=true, walks linked " +
			"child pages within the workspace's allowed sources up to max_depth/max_pages.",
		Parameters: map[string]*gollem.Parameter{
			"page_id": {
				Type:        gollem.TypeString,
				Description: "Notion page ID or full URL.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"recursive": {
				Type:        gollem.TypeBoolean,
				Description: "If true, recursively fetch linked child pages within scope.",
			},
			"max_depth": {
				Type:        gollem.TypeInteger,
				Description: "Maximum recursion depth (default 2, max 4).",
			},
			"max_pages": {
				Type:        gollem.TypeInteger,
				Description: "Maximum total pages to fetch (default 20, max 50).",
			},
		},
	}
}

func (t *getPageTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	rawID, err := argsutil.String(args, "page_id", true)
	if err != nil {
		return nil, err
	}
	_, id, err := notionsvc.ParseURL(rawID)
	if err != nil {
		return nil, goerr.Wrap(err, "invalid notion page id/url")
	}
	recursive := boolArg(args, "recursive")
	maxDepth := boundedInt(argsutil.Int(args, "max_depth"), getPageDefaultDepth, 1, getPageMaxDepth)
	maxPages := boundedInt(argsutil.Int(args, "max_pages"), getPageDefaultPages, 1, getPageMaxPages)

	if err := t.guard.Authorize(ctx, wsID, types.NotionObjectPage, id); err != nil {
		return nil, goerr.Wrap(err, "notion page not in allowed sources",
			goerr.V("page_id", id))
	}

	visited := map[string]struct{}{}
	skipped := []map[string]any{}
	fetched := []string{}
	truncated := false

	var b strings.Builder
	type frame struct {
		id    string
		depth int
		title string
	}
	queue := []frame{{id: id, depth: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if _, dup := visited[cur.id]; dup {
			continue
		}
		visited[cur.id] = struct{}{}

		if len(fetched) >= maxPages {
			truncated = true
			break
		}

		pm, err := t.client.GetPageMarkdown(ctx, cur.id)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to get page markdown",
				goerr.V("page_id", cur.id))
		}

		title := cur.title
		if title == "" {
			if meta, mErr := t.client.RetrievePage(ctx, cur.id); mErr == nil {
				title = meta.Title
			}
		}

		if cur.depth > 0 {
			b.WriteString("\n\n## ")
			b.WriteString(safeTitle(title, cur.id))
			b.WriteString("\n\n")
		} else if title != "" {
			b.WriteString("# ")
			b.WriteString(title)
			b.WriteString("\n\n")
		}
		b.WriteString(pm.Markdown)
		fetched = append(fetched, cur.id)

		if !recursive || cur.depth >= maxDepth {
			continue
		}
		for _, child := range pm.ChildPageIDs {
			if _, dup := visited[child]; dup {
				continue
			}
			if err := t.guard.Authorize(ctx, wsID, types.NotionObjectPage, child); err != nil {
				skipped = append(skipped, map[string]any{
					"id":     child,
					"reason": "out_of_scope",
				})
				continue
			}
			queue = append(queue, frame{id: child, depth: cur.depth + 1})
		}
	}

	return map[string]any{
		"markdown":  b.String(),
		"fetched":   fetched,
		"skipped":   skipped,
		"truncated": truncated,
	}, nil
}

func safeTitle(title, id string) string {
	if title != "" {
		return title
	}
	return id
}

func boolArg(args map[string]any, key string) bool {
	v, ok := args[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func boundedInt(v, def, lo, hi int) int {
	if v <= 0 {
		v = def
	}
	if v < lo {
		v = lo
	}
	if v > hi {
		v = hi
	}
	return v
}
