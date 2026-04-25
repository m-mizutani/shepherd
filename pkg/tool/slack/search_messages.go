package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	argsutil "github.com/m-mizutani/shepherd/pkg/tool/internal/args"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

type searchMessagesTool struct {
	slack SlackTooler
}

func newSearchMessagesTool(s SlackTooler) gollem.Tool {
	return &searchMessagesTool{slack: s}
}

func (t *searchMessagesTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "slack_search_messages",
		Description: "Search Slack messages across channels the bot can access. Returns matches with channel, author, text, timestamp, and permalink. Use Slack search modifiers like 'from:@user', 'in:#channel', 'after:2026-01-01' to narrow results.",
		Parameters: map[string]*gollem.Parameter{
			"query": {
				Type:        gollem.TypeString,
				Description: "Slack search query (supports Slack modifiers).",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of matches to return. Defaults to 20, capped at 50.",
			},
			"sort": {
				Type:        gollem.TypeString,
				Description: "Sort order: 'score' (relevance, default) or 'timestamp' (newest first).",
				Enum:        []string{"score", "timestamp"},
			},
		},
	}
}

func (t *searchMessagesTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	query, err := argsutil.String(args, "query", true)
	if err != nil {
		return nil, err
	}
	limit := clamp.Limit(argsutil.Int(args, "limit"), searchDefaultLimit, searchMaxLimit)
	sort, _ := argsutil.String(args, "sort", false)

	matches, err := t.slack.SearchMessages(ctx, query, limit, sort)
	if err != nil {
		return nil, goerr.Wrap(err, "slack_search_messages failed",
			goerr.V("query", query))
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}

	out := make([]map[string]any, 0, len(matches))
	for _, m := range matches {
		out = append(out, format.SlackSearchMatch(m))
	}
	return map[string]any{
		"matches": out,
		"count":   len(out),
	}, nil
}
