package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/clamp"
	"github.com/m-mizutani/shepherd/pkg/tool/internal/format"
)

const (
	threadDefaultLimit = 50
	threadMaxLimit     = 200
)

type getThreadTool struct {
	slack SlackTooler
}

func newGetThreadTool(s SlackTooler) gollem.Tool {
	return &getThreadTool{slack: s}
}

func (t *getThreadTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "slack_get_thread",
		Description: "Fetch all messages in a Slack thread (the parent message and its replies). Use this to read the full conversation context behind a ticket or a search hit.",
		Parameters: map[string]*gollem.Parameter{
			"channel_id": {
				Type:        gollem.TypeString,
				Description: "Slack channel ID, e.g. 'C0123456'.",
				Required:    true,
				MinLength:   ptrInt(1),
			},
			"thread_ts": {
				Type:        gollem.TypeString,
				Description: "Timestamp of the thread's root message.",
				Required:    true,
				MinLength:   ptrInt(1),
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of messages to return. Defaults to 50, capped at 200.",
			},
		},
	}
}

func (t *getThreadTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	channelID, err := stringArg(args, "channel_id", true)
	if err != nil {
		return nil, err
	}
	threadTS, err := stringArg(args, "thread_ts", true)
	if err != nil {
		return nil, err
	}
	limit := clamp.Limit(intArg(args, "limit"), threadDefaultLimit, threadMaxLimit)

	msgs, err := t.slack.GetThreadMessages(ctx, channelID, threadTS, limit)
	if err != nil {
		return nil, goerr.Wrap(err, "slack_get_thread failed",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS))
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
