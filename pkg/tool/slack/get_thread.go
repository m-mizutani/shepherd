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
		Description: "Fetch the full conversation in a Slack thread (parent message plus replies). " +
			"Args: `channel_id` (required, e.g. 'C0123456'); `thread_ts` (required, the root message's timestamp); optional `limit` (default 50, max 200). " +
			"Returns `{ messages: [{user, text, timestamp, thread_ts, bot_id}], count }`.",
		Parameters: map[string]*gollem.Parameter{
			"channel_id": {
				Type:        gollem.TypeString,
				Description: "Slack channel ID, e.g. 'C0123456'.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"thread_ts": {
				Type:        gollem.TypeString,
				Description: "Timestamp of the thread's root message.",
				Required:    true,
				MinLength:   argsutil.PtrInt(1),
			},
			"limit": {
				Type:        gollem.TypeInteger,
				Description: "Maximum number of messages to return. Defaults to 50, capped at 200.",
			},
		},
	}
}

func (t *getThreadTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	channelID, err := argsutil.String(args, "channel_id", true)
	if err != nil {
		return nil, err
	}
	threadTS, err := argsutil.String(args, "thread_ts", true)
	if err != nil {
		return nil, err
	}
	limit := clamp.Limit(argsutil.Int(args, "limit"), threadDefaultLimit, threadMaxLimit)

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
