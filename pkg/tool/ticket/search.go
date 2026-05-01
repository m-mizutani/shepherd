package ticket

import (
	"context"
	"sort"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	searchDefaultLimit = 10
	searchMaxLimit     = 30
)

// searchTool exposes semantic ticket search to the planner. The query is
// embedded once via the Gemini-backed Embedder and matched with cosine
// distance against the workspace's tickets in Firestore (FindNearest) or
// the in-memory brute-force fallback. When the caller passes an empty
// query the tool degrades to "list newest tickets in this workspace,
// optionally filtered by status".
type searchTool struct {
	repo     interfaces.Repository
	embedder interfaces.Embedder
}

func newSearchTool(r interfaces.Repository, e interfaces.Embedder) gollem.Tool {
	return &searchTool{repo: r, embedder: e}
}

func (t *searchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name: "ticket_search",
		Description: "Search past tickets in the active workspace by semantic similarity over title, description, and conclusion. " +
			"Args: optional `query` (free-form text; empty means list newest tickets without similarity ranking); optional `status_ids` (array of status IDs to whitelist; empty = all statuses); optional `limit` (default 10, max 30). " +
			"Returns `{ tickets: [{id, seq_num, title, status_id, status_name, distance, updated_at}], count }`. " +
			"`distance` is cosine distance (0 = identical, 2 = opposite); when `query` is empty the field is 0 and results are sorted by recent updates instead. Use this to find precedents and similar past incidents.",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Free-form description of what you are looking for. Empty means no semantic ranking; only the status filter applies and results come back sorted by recency.",
			},
			"status_ids": {
				Type:        gollem.TypeArray,
				Description: "Restrict results to tickets in these status IDs. Empty means all statuses.",
				Items:       &gollem.Parameter{Type: gollem.TypeString},
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of results. Defaults to 10, capped at 30.",
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

	if query == "" {
		return t.runRecency(ctx, wsID, statusIDs, limit)
	}
	return t.runSemantic(ctx, wsID, statusIDs, limit, query)
}

// runRecency falls back to the workspace-wide List when the caller does not
// provide a query. The behaviour mirrors the pre-embedding ticket_search
// shape (newest first, status filter respected) so callers that just want
// "latest tickets in this workspace" keep working without an embedding API
// call.
func (t *searchTool) runRecency(ctx context.Context, wsID types.WorkspaceID, statusIDs []types.StatusID, limit int) (map[string]any, error) {
	tickets, err := t.repo.Ticket().List(ctx, wsID, statusIDs)
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_search list failed")
	}
	sort.Slice(tickets, func(i, j int) bool {
		return tickets[i].UpdatedAt.After(tickets[j].UpdatedAt)
	})
	if len(tickets) > limit {
		tickets = tickets[:limit]
	}
	out := make([]map[string]any, 0, len(tickets))
	for _, tk := range tickets {
		row := format.TicketSummary(tk, "")
		row["distance"] = 0.0
		out = append(out, row)
	}
	return map[string]any{
		"tickets": out,
		"count":   len(out),
	}, nil
}

// runSemantic embeds the query and asks the repository for the nearest
// tickets. Embedding API failures are surfaced as a soft error in the
// returned payload (rather than as a Go error) so the agent loop can
// switch tactics — getting a transient embedding outage to abort the
// whole triage run would be heavy-handed.
func (t *searchTool) runSemantic(ctx context.Context, wsID types.WorkspaceID, statusIDs []types.StatusID, limit int, query string) (map[string]any, error) {
	if t.embedder == nil {
		return map[string]any{
			"error":   "ticket_search semantic mode is unavailable: embedding service not configured",
			"tickets": []any{},
			"count":   0,
		}, nil
	}
	vec, _, err := t.embedder.Generate(ctx, query)
	if err != nil {
		return map[string]any{
			"error":   "embedding call failed: " + err.Error(),
			"tickets": []any{},
			"count":   0,
		}, nil
	}
	hits, err := t.repo.Ticket().FindSimilar(ctx, wsID, vec, limit, statusIDs)
	if err != nil {
		return nil, goerr.Wrap(err, "ticket_search find similar failed")
	}

	out := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		row := format.TicketSummary(h.Ticket, "")
		row["distance"] = h.Distance
		out = append(out, row)
	}
	return map[string]any{
		"tickets": out,
		"count":   len(out),
	}, nil
}
