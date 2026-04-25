package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	historyDefaultLimit = 50
	historyMaxLimit     = 200
)

type getChannelHistoryTool struct {
	slack SlackTooler
}

func newGetChannelHistoryTool(s SlackTooler) gollem.Tool {
	return &getChannelHistoryTool{slack: s}
}

func (t *getChannelHistoryTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "slack_get_channel_history",
		Description: "Fetch recent top-level messages from a Slack channel (not thread replies). Use 'oldest' / 'latest' Slack timestamps to bracket a time range, e.g. when investigating activity around an incident.",
		Parameters: map[string]*gollem.Parameter{
			"channel_id": {
				Type:        gollem.TypeString,
				Description: "Slack channel ID, e.g. 'C0123456'.",
				Required:    true,
				MinLength:   ptrInt(1),
			},
			"oldest": {
				Type:        gollem.TypeString,
				Description: "Inclusive lower bound Slack timestamp (e.g. '1700000000.000000'). Empty means no lower bound.",
			},
			"latest": {
				Type:        gollem.TypeString,
				Description: "Inclusive upper bound Slack timestamp. Empty means no upper bound.",
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of messages to return. Defaults to 50, capped at 200.",
			},
		},
	}
}

func (t *getChannelHistoryTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	channelID, err := stringArg(args, "channel_id", true)
	if err != nil {
		return nil, err
	}
	oldest, _ := stringArg(args, "oldest", false)
	latest, _ := stringArg(args, "latest", false)
	limit := clamp.Limit(intArg(args, "limit"), historyDefaultLimit, historyMaxLimit)

	msgs, err := t.slack.GetChannelHistory(ctx, channelID, oldest, latest, limit)
	if err != nil {
		return nil, goerr.Wrap(err, "slack_get_channel_history failed",
			goerr.V("channel_id", channelID))
	}
	if len(msgs) > limit {
		msgs = msgs[:limit]
	}

	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, format.SlackMessage(m))
	}
	return map[string]any{
		"messages": out,
		"count":    len(out),
	}, nil
}
