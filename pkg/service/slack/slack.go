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

type UserInfo struct {
	Name  string
	Email string
}

func (c *Client) GetUserInfo(ctx context.Context, userID string) (*UserInfo, error) {
	user, err := c.api.GetUserInfoContext(ctx, userID)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get slack user info",
			goerr.V("user_id", userID),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	name := user.Profile.DisplayName
	if name == "" {
		name = user.RealName
	}
	if name == "" {
		name = user.Name
	}
	return &UserInfo{
		Name:  name,
		Email: user.Profile.Email,
	}, nil
}

func (c *Client) ReplyTicketCreated(ctx context.Context, channelID, threadTS string, seqNum int64, ticketURL string) error {
	text := fmt.Sprintf("<%s|Ticket #%d> created", ticketURL, seqNum)
	return c.ReplyThread(ctx, channelID, threadTS, text)
}

func (c *Client) ReplyStatusChange(ctx context.Context, channelID, threadTS, oldStatusName, newStatusName string) error {
	text := fmt.Sprintf("Status: *%s* → *%s*", oldStatusName, newStatusName)
	_, _, err := c.api.PostMessageContext(ctx, channelID,
		slackgo.MsgOptionTS(threadTS),
		slackgo.MsgOptionBlocks(
			slackgo.NewContextBlock("",
				slackgo.NewTextBlockObject("mrkdwn", text, false, false),
			),
		),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to post status change notification",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}

func (c *Client) ResolveChannelName(ctx context.Context, name string) (string, error) {
	var cursor string
	for {
		params := &slackgo.GetConversationsParameters{
			Cursor:          cursor,
			Limit:           200,
			Types:           []string{"public_channel", "private_channel"},
			ExcludeArchived: true,
		}
		channels, nextCursor, err := c.api.GetConversationsContext(ctx, params)
		if err != nil {
			return "", goerr.Wrap(err, "failed to list slack channels",
				goerr.V("channel_name", name),
				goerr.Tag(errutil.TagSlackError),
			)
		}
		for _, ch := range channels {
			if ch.Name == name {
				return ch.ID, nil
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return "", goerr.New("slack channel not found",
		goerr.V("channel_name", name),
		goerr.Tag(errutil.TagSlackError),
	)
}
