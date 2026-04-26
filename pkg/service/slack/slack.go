package slack

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/utils/errutil"
	"github.com/m-mizutani/shepherd/pkg/utils/i18n"
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

// PostThreadBlocks posts a Slack message containing arbitrary Block Kit
// blocks into a thread, returning the new message's timestamp so callers
// can subsequently update or reference it.
func (c *Client) PostThreadBlocks(ctx context.Context, channelID, threadTS string, blocks []slackgo.Block) (string, error) {
	_, ts, err := c.api.PostMessageContext(ctx, channelID,
		slackgo.MsgOptionTS(threadTS),
		slackgo.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		return "", goerr.Wrap(err, "failed to post slack thread blocks",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return ts, nil
}

// UpdateMessage rewrites an existing Slack message with the supplied Block
// Kit blocks. Used to mutate triage progress / question messages in place.
func (c *Client) UpdateMessage(ctx context.Context, channelID, messageTS string, blocks []slackgo.Block) error {
	_, _, _, err := c.api.UpdateMessageContext(ctx, channelID, messageTS,
		slackgo.MsgOptionBlocks(blocks...),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to update slack message",
			goerr.V("channel_id", channelID),
			goerr.V("message_ts", messageTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}

// PostEphemeral posts a message visible only to the supplied user inside a
// channel. Used for transient triage error feedback (e.g. "this form is no
// longer valid") when chat.update is not appropriate.
func (c *Client) PostEphemeral(ctx context.Context, channelID, userID, text string) error {
	_, err := c.api.PostEphemeralContext(ctx, channelID, userID,
		slackgo.MsgOptionText(text, false),
	)
	if err != nil {
		return goerr.Wrap(err, "failed to post ephemeral slack message",
			goerr.V("channel_id", channelID),
			goerr.V("user_id", userID),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return nil
}

type UserInfo struct {
	ID       string
	Name     string
	Email    string
	ImageURL string
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
		ID:       userID,
		Name:     name,
		Email:    user.Profile.Email,
		ImageURL: user.Profile.Image48,
	}, nil
}

func (c *Client) ListUsers(ctx context.Context) ([]*UserInfo, error) {
	var result []*UserInfo
	p := c.api.GetUsersPaginated(slackgo.GetUsersOptionLimit(200))
	for {
		var err error
		p, err = p.Next(ctx)
		if err != nil {
			if p.Done(err) {
				break
			}
			return nil, goerr.Wrap(err, "failed to list slack users",
				goerr.Tag(errutil.TagSlackError),
			)
		}

		for _, u := range p.Users {
			if u.Deleted || u.IsBot {
				continue
			}
			name := u.Profile.DisplayName
			if name == "" {
				name = u.RealName
			}
			if name == "" {
				name = u.Name
			}
			result = append(result, &UserInfo{
				ID:       u.ID,
				Name:     name,
				Email:    u.Profile.Email,
				ImageURL: u.Profile.Image48,
			})
		}
	}
	return result, nil
}

func (c *Client) ReplyTicketCreated(ctx context.Context, channelID, threadTS string, seqNum int64, ticketURL string) error {
	text := i18n.From(ctx).T(i18n.MsgTicketCreated, "url", ticketURL, "id", seqNum)
	return c.ReplyThread(ctx, channelID, threadTS, text)
}

func (c *Client) ReplyStatusChange(ctx context.Context, channelID, threadTS, oldStatusName, newStatusName string) error {
	text := i18n.From(ctx).T(i18n.MsgStatusChange, "old", oldStatusName, "new", newStatusName)
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

// Message is a minimal representation of a Slack message used by tools and
// downstream consumers. It hides the noisy slack-go Message struct behind a
// small surface of fields that LLMs actually care about.
type Message struct {
	User      string
	Text      string
	Timestamp string
	ThreadTS  string
	BotID     string
}

// SearchMatch is one hit returned by the search.messages API.
type SearchMatch struct {
	ChannelID   string
	ChannelName string
	User        string
	Username    string
	Text        string
	Timestamp   string
	Permalink   string
}

// SearchMessages calls the Slack search.messages API.
// count is clamped to slack-go defaults when zero. sort is "score" (default) or "timestamp".
func (c *Client) SearchMessages(ctx context.Context, query string, count int, sort string) ([]*SearchMatch, error) {
	params := slackgo.NewSearchParameters()
	if count > 0 {
		params.Count = count
	}
	if sort != "" {
		params.Sort = sort
	}
	res, _, err := c.api.SearchContext(ctx, query, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to search slack messages",
			goerr.V("query", query),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	matches := make([]*SearchMatch, 0, len(res.Matches))
	for _, m := range res.Matches {
		matches = append(matches, &SearchMatch{
			ChannelID:   m.Channel.ID,
			ChannelName: m.Channel.Name,
			User:        m.User,
			Username:    m.Username,
			Text:        m.Text,
			Timestamp:   m.Timestamp,
			Permalink:   m.Permalink,
		})
	}
	return matches, nil
}

// GetThreadMessages returns the messages of a thread (root + replies).
func (c *Client) GetThreadMessages(ctx context.Context, channelID, threadTS string, limit int) ([]*Message, error) {
	params := &slackgo.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTS,
	}
	if limit > 0 {
		params.Limit = limit
	}
	msgs, _, _, err := c.api.GetConversationRepliesContext(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get slack thread replies",
			goerr.V("channel_id", channelID),
			goerr.V("thread_ts", threadTS),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return convertMessages(msgs), nil
}

// GetChannelHistory returns recent messages in a channel.
// oldest/latest are RFC-style Slack TS strings. Empty values mean unbounded.
func (c *Client) GetChannelHistory(ctx context.Context, channelID, oldest, latest string, limit int) ([]*Message, error) {
	params := &slackgo.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    oldest,
		Latest:    latest,
	}
	if limit > 0 {
		params.Limit = limit
	}
	res, err := c.api.GetConversationHistoryContext(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get slack channel history",
			goerr.V("channel_id", channelID),
			goerr.Tag(errutil.TagSlackError),
		)
	}
	return convertMessages(res.Messages), nil
}

func convertMessages(msgs []slackgo.Message) []*Message {
	out := make([]*Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, &Message{
			User:      m.User,
			Text:      m.Text,
			Timestamp: m.Timestamp,
			ThreadTS:  m.ThreadTimestamp,
			BotID:     m.BotID,
		})
	}
	return out
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
