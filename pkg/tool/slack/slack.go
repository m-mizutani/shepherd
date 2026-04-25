// Package slack contains gollem.Tool implementations that read Slack data
// (search, thread, channel history, user info). The tools delegate API calls
// to a SlackTooler interface so tests can substitute a fake.
package slack

import (
	"context"

	"github.com/m-mizutani/gollem"
	slackService "github.com/m-mizutani/shepherd/pkg/service/slack"
)

// SlackTooler is the subset of the Slack service used by the tools in this
// package. The parent pkg/tool defines the same shape and re-exports it.
type SlackTooler interface {
	SearchMessages(ctx context.Context, query string, count int, sort string) ([]*slackService.SearchMatch, error)
	GetThreadMessages(ctx context.Context, channelID, threadTS string, limit int) ([]*slackService.Message, error)
	GetChannelHistory(ctx context.Context, channelID, oldest, latest string, limit int) ([]*slackService.Message, error)
	GetUserInfo(ctx context.Context, userID string) (*slackService.UserInfo, error)
}

// Deps bundles dependencies for Slack-facing tools.
type Deps struct {
	Slack SlackTooler
}

// Tools returns every gollem.Tool exported from this package.
func Tools(d Deps) []gollem.Tool {
	return []gollem.Tool{
		newSearchMessagesTool(d.Slack),
		newGetThreadTool(d.Slack),
		newGetChannelHistoryTool(d.Slack),
		newGetUserInfoTool(d.Slack),
	}
}
