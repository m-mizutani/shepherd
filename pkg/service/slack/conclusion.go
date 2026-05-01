package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
	slackgo "github.com/slack-go/slack"
)

// PostConclusion posts the LLM-generated close-time conclusion into the
// originating ticket thread as a single Slack context block.
//
// The output is intentionally low-key: one context block, plain prose,
// and exactly one decorative emoji (rendered server-side via the i18n
// template) so close summaries blend into the thread rather than
// competing with it. Callers must ensure the thread coordinates are
// non-empty; an empty body collapses to a no-op.
func (c *Client) PostConclusion(ctx context.Context, channelID, threadTS, conclusion string) error {
	if conclusion == "" {
		return nil
	}
	body := i18n.From(ctx).T(i18n.MsgConclusionBody, "conclusion", conclusion)

	_, _, err := c.api.PostMessageContext(ctx, channelID,
		slackgo.MsgOptionTS(threadTS),
		slackgo.MsgOptionBlocks(
			slackgo.NewContextBlock("",
				slackgo.NewTextBlockObject("mrkdwn", body, false, false),
			),
		),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to post ticket conclusion",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}
