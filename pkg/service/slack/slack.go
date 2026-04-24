package slack

import (
	"context"
	"fmt"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	slackgo "github.com/slack-go/slack"
)

type Client struct {
	api *slackgo.Client
}

func NewClient(botToken string) *Client {
	return &Client{
		api: slackgo.New(botToken),
	}
}

func (c *Client) PostMessage(ctx context.Context, channelID, text string) (string, string, error) {
	ch, ts, err := c.api.PostMessageContext(ctx, channelID, slackgo.MsgOptionText(text, false))
	if err != nil {
		return "", "", goerr.Wrap(err, "failed to post slack message",
			goerr.V("channel_id", channelID),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return ch, ts, nil
}

func (c *Client) ReplyThread(ctx context.Context, channelID, threadTS, text string) error {
	_, _, err := c.api.PostMessageContext(ctx, channelID,
		slackgo.MsgOptionText(text, false),
		slackgo.MsgOptionTS(threadTS),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to reply in slack thread",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}

func (c *Client) ReplyTicketCreated(ctx context.Context, channelID, threadTS string, seqNum int64, ticketURL string) error {
	text := fmt.Sprintf("Ticket #%d created: %s", seqNum, ticketURL)
	return c.ReplyThread(ctx, channelID, threadTS, text)
}
