package ticket

import (
	"context"
	"sort"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

type searchTool struct {
	repo interfaces.Repository
}

func newSearchTool(r interfaces.Repository) gollem.Tool {
	return &searchTool{repo: r}
}

func (t *searchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "ticket_search",
		Description: "Search past tickets in the active workspace by case-insensitive substring match against title and description. Optionally filter by status IDs. Results are sorted newest-updated first.",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Substring to look for in title or description. Empty means no text filter (only status filter applies).",
			},
			"status_ids": {
				Type:        gollem.TypeArray,
				Description: "Restrict results to tickets in these status IDs. Empty means all statuses.",
				Items:       &gollem.Parameter{Type: gollem.TypeString},
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of results. Defaults to 20, capped at 50.",
			},
		},
	}
}

func (t *searchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	wsID, err := workspaceFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	query, _ := argsutil.String(args, "query", false)
	statuses := argsutil.StringSlice(args, "status_ids")
	limit := clamp.Limit(argsutil.Int(args, "limit"), searchDefaultLimit, searchMaxLimit)

	statusIDs := make([]types.StatusID, 0, len(statuses))
	for _, s := range statuses {
		statusIDs = append(statusIDs, types.StatusID(s))
	}

	tickets, err := t.repo.Ticket().List(ctx, wsID, statusIDs)
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_search list failed")
	}

	q := strings.ToLower(query)
	matched := tickets[:0]
	for _, tk := range tickets {
		if q != "" && !strings.Contains(strings.ToLower(tk.Title), q) && !strings.Contains(strings.ToLower(tk.Description), q) {
			continue
		}
		matched = append(matched, tk)
	}
	sort.Slice(matched, func(i, j int) bool {
		return matched[i].UpdatedAt.After(matched[j].UpdatedAt)
	})
	if len(matched) > limit {
		matched = matched[:limit]
	}

	out := make([]map[string]any, 0, len(matched))
	for _, tk := range matched {
		out = append(out, format.TicketSummary(tk, ""))
	}
	return map[string]any{
		"tickets": out,
		"count":   len(out),
	}, nil
}
